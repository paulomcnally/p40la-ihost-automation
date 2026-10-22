package services

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	exportAppName       = "p40la-ihost-automation"
	exportFormatVersion = 1
)

// exportTable describe una tabla incluida en el export y la columna usada para
// ordenar las filas (determinista, facilita diff entre exports).
type exportTable struct {
	name  string
	order string
}

// exportTables es la lista blanca de tablas exportadas. schema_migrations se
// exporta como metadato (lista de versiones), no como tabla de datos.
var exportTables = []exportTable{
	{name: "users", order: "id"},
	{name: "settings", order: "key"},
	{name: "apps", order: "id"},
	{name: "accounts", order: "id"},
	{name: "plugins", order: "id"},
	{name: "bills", order: "id"},
	{name: "webhook_logs", order: "id"},
}

// exportJSONColumns indica qué columnas contienen JSON anidado y deben
// embeberse como objeto/array cuando el contenido es válido (REQ-011).
var exportJSONColumns = map[string]map[string]bool{
	"accounts":     {"credentials": true},
	"bills":        {"raw": true},
	"webhook_logs": {"payload": true, "response": true},
}

// ExportOptions controla el formato del documento exportado.
type ExportOptions struct {
	// Pretty indenta el JSON (más bytes, más legible).
	Pretty bool
}

// ExportStats resume un export para la auditoría en logs.
type ExportStats struct {
	Tables int
	Rows   int64
	Bytes  int64
}

// ExportService genera un volcado JSON de toda la base de datos en streaming.
type ExportService struct {
	db *sql.DB
}

// NewExportService crea un nuevo ExportService.
func NewExportService(db *sql.DB) *ExportService {
	return &ExportService{db: db}
}

type exportMetadata struct {
	FormatVersion    int              `json:"format_version"`
	App              string           `json:"app"`
	ExportedAt       string           `json:"exported_at"`
	ExportedBy       string           `json:"exported_by"`
	SchemaMigrations []string         `json:"schema_migrations"`
	Counts           map[string]int64 `json:"counts"`
}

// Stream escribe el export completo en dst. No materializa el documento en
// memoria: cada tabla se emite fila a fila con json.Encoder (ADR-002).
//
// Si devuelve error y stats.Bytes == 0, todavía no se escribió nada y el
// llamador puede responder un error HTTP normal; si stats.Bytes > 0 la
// respuesta ya está en curso y solo queda registrar el fallo.
func (s *ExportService) Stream(ctx context.Context, dst io.Writer, exportedBy string, opts ExportOptions) (ExportStats, error) {
	var stats ExportStats

	// Preflight antes de escribir el primer byte: migraciones y conteos.
	migrations, err := s.schemaMigrations(ctx)
	if err != nil {
		return stats, err
	}

	counts := make(map[string]int64, len(exportTables))
	for _, table := range exportTables {
		var count int64
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table.name).Scan(&count); err != nil {
			return stats, fmt.Errorf("contar %s: %w", table.name, err)
		}
		counts[table.name] = count
	}

	buf := bufio.NewWriterSize(dst, 32*1024)
	defer func() { _ = buf.Flush() }()

	counter := &countingWriter{w: buf}
	var out io.Writer = counter
	if opts.Pretty {
		out = &prettyJSONWriter{w: counter}
	}

	meta := exportMetadata{
		FormatVersion:    exportFormatVersion,
		App:              exportAppName,
		ExportedAt:       time.Now().UTC().Format(time.RFC3339),
		ExportedBy:       exportedBy,
		SchemaMigrations: migrations,
		Counts:           counts,
	}
	metaJSON, err := marshalNoEscape(meta)
	if err != nil {
		return stats, fmt.Errorf("serializar metadatos: %w", err)
	}

	// Se recortan las llaves externas del metadata para anidar "tables" dentro
	// del mismo objeto "export".
	if _, err := io.WriteString(out, `{"export":{`); err != nil {
		return stats, err
	}
	if _, err := out.Write(metaJSON[1 : len(metaJSON)-1]); err != nil {
		return stats, err
	}
	if _, err := io.WriteString(out, `,"tables":{`); err != nil {
		return stats, err
	}

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	for i, table := range exportTables {
		if i > 0 {
			if _, err := io.WriteString(out, ","); err != nil {
				return stats, err
			}
		}
		if _, err := io.WriteString(out, `"`+table.name+`":[`); err != nil {
			return stats, err
		}
		rows, err := s.dumpTable(ctx, out, enc, table)
		if err != nil {
			return stats, fmt.Errorf("exportar %s: %w", table.name, err)
		}
		if _, err := io.WriteString(out, "]"); err != nil {
			return stats, err
		}
		stats.Tables++
		stats.Rows += rows
	}

	if _, err := io.WriteString(out, "}}}"); err != nil {
		return stats, err
	}

	if err := buf.Flush(); err != nil {
		return stats, err
	}
	stats.Bytes = counter.n
	return stats, nil
}

// dumpTable emite las filas de una tabla como elementos de un array JSON.
func (s *ExportService) dumpTable(ctx context.Context, w io.Writer, enc *json.Encoder, table exportTable) (int64, error) {
	query := "SELECT * FROM " + table.name
	if table.order != "" {
		query += " ORDER BY " + table.order
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	values := make([]any, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}

	var count int64
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return count, err
		}

		record := make(map[string]any, len(columns))
		for i, column := range columns {
			record[column] = normalizeExportValue(table.name, column, values[i])
		}

		if count > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return count, err
			}
		}
		if err := enc.Encode(record); err != nil {
			return count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	return count, nil
}

// schemaMigrations devuelve las migraciones aplicadas, en orden.
func (s *ExportService) schemaMigrations(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("listar migraciones: %w", err)
	}
	defer rows.Close()

	migrations := make([]string, 0, 8)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("escanear migración: %w", err)
		}
		migrations = append(migrations, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorrer migraciones: %w", err)
	}
	return migrations, nil
}

// normalizeExportValue adapta los tipos que devuelve el driver a valores JSON.
func normalizeExportValue(table, column string, value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case []byte:
		return normalizeExportJSONText(table, column, string(v))
	case string:
		return normalizeExportJSONText(table, column, v)
	default:
		return v
	}
}

// normalizeExportJSONText embebe JSON válido como objeto/array y deja el resto
// como string, sin perder información cuando el contenido no es JSON.
func normalizeExportJSONText(table, column, text string) any {
	if !exportJSONColumns[table][column] {
		return text
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return text
	}
	if !json.Valid([]byte(trimmed)) {
		return text
	}
	return json.RawMessage(trimmed)
}

// marshalNoEscape serializa sin escapar caracteres HTML para no alterar valores.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// countingWriter cuenta los bytes escritos hacia el buffer.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// prettyJSONWriter reindenta en streaming el JSON compacto que recibe, sin
// bufferear el documento completo. Se apoya en que el productor solo emite JSON
// válido y compacto (sin espacios fuera de strings).
type prettyJSONWriter struct {
	w             io.Writer
	inString      bool
	escaped       bool
	depth         int
	pendingIndent bool
	lastByte      byte
}

func (p *prettyJSONWriter) Write(data []byte) (int, error) {
	for _, c := range data {
		if p.inString {
			if _, err := p.w.Write([]byte{c}); err != nil {
				return 0, err
			}
			if p.escaped {
				p.escaped = false
			} else if c == '\\' {
				p.escaped = true
			} else if c == '"' {
				p.inString = false
			}
			continue
		}

		switch c {
		case ' ', '\n', '\t', '\r':
			continue
		case '"':
			if err := p.writeIndent(); err != nil {
				return 0, err
			}
			p.inString = true
			p.lastByte = c
		case '{', '[':
			if err := p.writeIndent(); err != nil {
				return 0, err
			}
			p.depth++
			p.lastByte = c
			p.pendingIndent = true
			if _, err := p.w.Write([]byte{c}); err != nil {
				return 0, err
			}
			continue
		case '}', ']':
			p.depth--
			empty := (p.lastByte == '{' && c == '}') || (p.lastByte == '[' && c == ']')
			if !empty {
				if err := p.writeNewline(p.depth); err != nil {
					return 0, err
				}
			}
			p.lastByte = c
			p.pendingIndent = false
			if _, err := p.w.Write([]byte{c}); err != nil {
				return 0, err
			}
			continue
		case ',':
			if _, err := p.w.Write([]byte{c}); err != nil {
				return 0, err
			}
			if err := p.writeNewline(p.depth); err != nil {
				return 0, err
			}
			p.lastByte = c
			p.pendingIndent = false
			continue
		case ':':
			if _, err := p.w.Write([]byte(": ")); err != nil {
				return 0, err
			}
			p.lastByte = c
			p.pendingIndent = false
			continue
		default:
			if err := p.writeIndent(); err != nil {
				return 0, err
			}
			if _, err := p.w.Write([]byte{c}); err != nil {
				return 0, err
			}
			p.lastByte = c
			continue
		}

		// Para comillas: escribir el carácter y limpiar la indentación pendiente.
		p.pendingIndent = false
		if _, err := p.w.Write([]byte{'"'}); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func (p *prettyJSONWriter) writeIndent() error {
	if !p.pendingIndent {
		return nil
	}
	p.pendingIndent = false
	return p.writeNewline(p.depth)
}

func (p *prettyJSONWriter) writeNewline(depth int) error {
	if _, err := io.WriteString(p.w, "\n"); err != nil {
		return err
	}
	if depth > 0 {
		if _, err := io.WriteString(p.w, strings.Repeat("  ", depth)); err != nil {
			return err
		}
	}
	return nil
}
