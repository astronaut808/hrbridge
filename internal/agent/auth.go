package agent

import (
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ConfigPath == "" {
		writeError(w, http.StatusInternalServerError, "config path is not available")
		return
	}
	token, err := generateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg.AuthToken = token
	if err := writeConfig(s.cfg.ConfigPath, s.cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.TokenRotateResponse{Token: token})
}
