package agent

import (
	"encoding/json"
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleGetCIDRStructured(w http.ResponseWriter, r *http.Request) {
	cfg, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.CIDRList))
	writeJSON(w, http.StatusOK, openapi.CIDRConfigResponse{
		Path:   s.cfg.CIDRList,
		Config: cfg,
	})
}

func (s *Server) handlePutCIDRStructured(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutCIDRConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "config.write.structured", "cidr", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCIDRConfig(req.Config); err != nil {
		s.writeAudit(r, "config.write.structured", "cidr", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !checkRevisionPrecondition(r, s.cfg.CIDRList) {
		writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
		return
	}

	backup, err := s.createBackup(configBackupReason("cidr-structured"))
	if err != nil {
		s.writeAudit(r, "config.write.structured", "cidr", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.CIDRList, []byte(renderCIDRConfig(req.Config)), 0o600); err != nil {
		s.writeAudit(r, "config.write.structured", "cidr", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.CIDRList))

	resp := openapi.PutConfigResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.PutConfigResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "config.write.structured", "cidr", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "config.write.structured", "cidr", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}
