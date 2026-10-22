package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// ExportHandlers agrupa los handlers del respaldo completo de la base de datos.
type ExportHandlers struct {
	service *services.ExportService
}

// NewExportHandlers crea un nuevo ExportHandlers.
func NewExportHandlers(service *services.ExportService) *ExportHandlers {
	return &ExportHandlers{service: service}
}

// ExportDatabase descarga toda la base de datos como un archivo JSON en texto
// plano (SPEC-018). La respuesta se genera en streaming.
func (h *ExportHandlers) ExportDatabase(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Sesión no válida")
		return
	}

	filename := fmt.Sprintf("p40la-export-%s.json", time.Now().UTC().Format("20060102T150405Z"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")

	stats, err := h.service.Stream(r.Context(), w, user.Email, services.ExportOptions{
		Pretty: r.URL.Query().Get("pretty") == "true",
	})
	if err != nil {
		// Si aún no se escribió nada, se puede responder un error JSON limpio.
		if stats.Bytes == 0 {
			respondError(w, http.StatusInternalServerError, "internal_error", "Error al exportar la base de datos: "+err.Error())
			return
		}
		// A mitad del stream solo queda registrar: el cliente recibe un JSON truncado.
		slog.Error("export interrumpido", "user", user.Email, "error", err, "bytes", stats.Bytes)
		return
	}

	slog.Info("export completado",
		"user", user.Email,
		"tables", stats.Tables,
		"rows", stats.Rows,
		"bytes", stats.Bytes,
	)
}
