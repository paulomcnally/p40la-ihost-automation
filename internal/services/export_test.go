package services

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/paulomcnally/p40la-ihost-automation/internal/db"
)

func newTestExportService(t *testing.T) (*ExportService, context.Context) {
	t.Helper()
	database, err := db.OpenDB(t.TempDir()+"/test.db", "../../migrations")
	if err != nil {
		t.Fatalf("abrir base de datos de prueba: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	ctx := context.Background()
	statements := []string{
		`INSERT INTO users (email, password_hash) VALUES ('admin@example.com', '$2a$10$hash-de-prueba')`,
		`INSERT INTO settings (key, value) VALUES ('language', 'es'), ('webhook.api_key', 'clave-secreta')`,
		`INSERT INTO apps (name) VALUES ('App de prueba')`,
		`INSERT INTO plugins (name, version, source, status) VALUES ('demo.nicaragua', '1.0.0', 'https://demo.ni', 'active')`,
		`INSERT INTO accounts (app_id, identifier, label, credentials, plugin_name, webhook_url, schedule_enabled)
		 VALUES (1, 'SVC-1', 'Cuenta demo', '{"username":"demo","password":"secreto"}', 'demo.nicaragua', 'https://hook.example/x', 1)`,
		`INSERT INTO bills (account_id, plugin_name, plugin_version, source, status, raw)
		 VALUES (1, 'demo.nicaragua', '1.0.0', 'https://demo.ni', 'ok', '{"year":2026,"amount":100.5}')`,
		`INSERT INTO webhook_logs (account_id, webhook_url, status, payload, response)
		 VALUES (1, 'https://hook.example/x', 'ok', '{"bills":[{"amount":"100.5"}]}', 'texto plano')`,
	}
	for _, stmt := range statements {
		if _, err := database.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("sembrar datos: %v", err)
		}
	}
	return NewExportService(database), ctx
}

func decodeExport(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, body)
	}
	root, ok := doc["export"].(map[string]any)
	if !ok {
		t.Fatalf("falta la llave export: %v", doc)
	}
	return root
}

func TestExportStreamIncluyeTodaLaData(t *testing.T) {
	svc, ctx := newTestExportService(t)

	var buf bytes.Buffer
	stats, err := svc.Stream(ctx, &buf, "admin@example.com", ExportOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if stats.Tables != len(exportTables) {
		t.Errorf("Stats.Tables = %d, se esperaba %d", stats.Tables, len(exportTables))
	}
	if stats.Bytes == 0 || stats.Bytes != int64(buf.Len()) {
		t.Errorf("Stats.Bytes = %d, buffer = %d", stats.Bytes, buf.Len())
	}

	root := decodeExport(t, buf.Bytes())

	if root["format_version"].(float64) != 1 {
		t.Errorf("format_version = %v", root["format_version"])
	}
	if root["app"] != exportAppName {
		t.Errorf("app = %v", root["app"])
	}
	if root["exported_by"] != "admin@example.com" {
		t.Errorf("exported_by = %v", root["exported_by"])
	}
	if _, err := json.Marshal(root["exported_at"]); err != nil {
		t.Errorf("exported_at inválido: %v", root["exported_at"])
	}

	migrations, ok := root["schema_migrations"].([]any)
	if !ok || len(migrations) == 0 {
		t.Fatalf("schema_migrations vacío: %v", root["schema_migrations"])
	}

	counts := root["counts"].(map[string]any)
	tables := root["tables"].(map[string]any)
	for _, table := range exportTables {
		rows, ok := tables[table.name].([]any)
		if !ok {
			t.Errorf("falta la tabla %s en el export", table.name)
			continue
		}
		if counts[table.name].(float64) != float64(len(rows)) {
			t.Errorf("counts[%s] = %v, filas = %d", table.name, counts[table.name], len(rows))
		}
	}
	if stats.Rows != 8 {
		t.Errorf("Stats.Rows = %d, se esperaban 8", stats.Rows)
	}
}

func TestExportStreamSecretosYJSONAnidado(t *testing.T) {
	svc, ctx := newTestExportService(t)

	var buf bytes.Buffer
	if _, err := svc.Stream(ctx, &buf, "admin@example.com", ExportOptions{}); err != nil {
		t.Fatalf("export: %v", err)
	}
	root := decodeExport(t, buf.Bytes())
	tables := root["tables"].(map[string]any)

	user := tables["users"].([]any)[0].(map[string]any)
	if user["password_hash"] != "$2a$10$hash-de-prueba" {
		t.Errorf("password_hash = %v", user["password_hash"])
	}

	account := tables["accounts"].([]any)[0].(map[string]any)
	credentials, ok := account["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("credentials no se embebió como objeto: %v", account["credentials"])
	}
	if credentials["password"] != "secreto" {
		t.Errorf("credencial = %v", credentials["password"])
	}

	bill := tables["bills"].([]any)[0].(map[string]any)
	if _, ok := bill["raw"].(map[string]any); !ok {
		t.Errorf("bills.raw no se embebió como objeto: %v", bill["raw"])
	}

	log := tables["webhook_logs"].([]any)[0].(map[string]any)
	if _, ok := log["payload"].(map[string]any); !ok {
		t.Errorf("payload no se embebió como objeto: %v", log["payload"])
	}
	if log["response"] != "texto plano" {
		t.Errorf("response (no JSON) se alteró: %v", log["response"])
	}

	settings := tables["settings"].([]any)
	keys := make(map[string]string, len(settings))
	for _, row := range settings {
		kv := row.(map[string]any)
		keys[kv["key"].(string)] = kv["value"].(string)
	}
	if keys["webhook.api_key"] != "clave-secreta" {
		t.Errorf("api_key no exportada en claro: %v", keys)
	}
}

func TestExportStreamJSONInvalidoSeConservaComoString(t *testing.T) {
	svc, ctx := newTestExportService(t)

	if _, err := svc.db.ExecContext(ctx,
		`INSERT INTO bills (account_id, plugin_name, plugin_version, source, status, raw)
		 VALUES (1, 'demo.nicaragua', '1.0.0', 'x', 'error', 'no-es-json')`); err != nil {
		t.Fatalf("insertar bill inválida: %v", err)
	}

	var buf bytes.Buffer
	if _, err := svc.Stream(ctx, &buf, "admin@example.com", ExportOptions{}); err != nil {
		t.Fatalf("export: %v", err)
	}
	root := decodeExport(t, buf.Bytes())
	bills := root["tables"].(map[string]any)["bills"].([]any)
	if len(bills) != 2 {
		t.Fatalf("bills = %d, se esperaban 2", len(bills))
	}
	if bills[1].(map[string]any)["raw"] != "no-es-json" {
		t.Errorf("raw inválido se alteró: %v", bills[1].(map[string]any)["raw"])
	}
}

func TestExportStreamPretty(t *testing.T) {
	svc, ctx := newTestExportService(t)

	var buf bytes.Buffer
	if _, err := svc.Stream(ctx, &buf, "admin@example.com", ExportOptions{Pretty: true}); err != nil {
		t.Fatalf("export pretty: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatalf("pretty JSON inválido:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "\n  \"export\":") {
		t.Errorf("el pretty no está indentado:\n%s", buf.String())
	}
	// La data debe ser idéntica al export compacto.
	var prettyDoc, compactDoc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &prettyDoc); err != nil {
		t.Fatalf("parse pretty: %v", err)
	}
	var compact bytes.Buffer
	if _, err := svc.Stream(ctx, &compact, "admin@example.com", ExportOptions{}); err != nil {
		t.Fatalf("export compacto: %v", err)
	}
	if err := json.Unmarshal(compact.Bytes(), &compactDoc); err != nil {
		t.Fatalf("parse compacto: %v", err)
	}
	prettyRoot := prettyDoc["export"].(map[string]any)
	compactRoot := compactDoc["export"].(map[string]any)
	delete(prettyRoot, "exported_at")
	delete(compactRoot, "exported_at")
	if !jsonEqual(t, prettyRoot, compactRoot) {
		t.Errorf("el contenido pretty difiere del compacto")
	}
}

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	aj, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	bj, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	return bytes.Equal(aj, bj)
}
