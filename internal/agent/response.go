package agent

import (
	"encoding/json"
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, openapi.ErrorResponse{Error: msg})
}
