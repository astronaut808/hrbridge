package agent

import (
	"encoding/json"
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleGetDomainsStructured(w http.ResponseWriter, r *http.Request) {
	cfg, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.DomainConf))
	writeJSON(w, http.StatusOK, openapi.DomainConfigResponse{
		Path:   s.cfg.DomainConf,
		Config: cfg,
	})
}

func (s *Server) handlePutDomainsStructured(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutDomainConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "config.write.structured", "domains", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDomainConfig(req.Config); err != nil {
		s.writeAudit(r, "config.write.structured", "domains", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !checkRevisionPrecondition(r, s.cfg.DomainConf) {
		writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
		return
	}

	backup, err := s.createBackup(configBackupReason("domains-structured"))
	if err != nil {
		s.writeAudit(r, "config.write.structured", "domains", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.DomainConf, []byte(renderDomainConfig(req.Config)), 0o600); err != nil {
		s.writeAudit(r, "config.write.structured", "domains", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.DomainConf))

	resp := openapi.PutConfigResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.PutConfigResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "config.write.structured", "domains", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "config.write.structured", "domains", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}
