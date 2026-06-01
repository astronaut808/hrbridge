package agent

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleGetHRNeoStructured(w http.ResponseWriter, r *http.Request) {
	cfg, unknown, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := openapi.HRNeoConfigResponse{
		Path:    s.cfg.HRNeoConf,
		Config:  cfg,
		Unknown: &unknown,
	}
	setRevisionHeader(w, fileRevision(s.cfg.HRNeoConf))
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePutHRNeoStructured(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutHRNeoConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "config.write.structured", "hrneo", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateHRNeoConfig(req.Config); err != nil {
		s.writeAudit(r, "config.write.structured", "hrneo", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !checkRevisionPrecondition(r, s.cfg.HRNeoConf) {
		writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
		return
	}

	current, unknown, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil {
		if !os.IsNotExist(err) {
			s.writeAudit(r, "config.write.structured", "hrneo", false, "", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		current = defaultHRNeoConfig()
		unknown = map[string]string{}
	}
	merged := mergeHRNeoConfig(current, req.Config)

	backup, err := s.createBackup(configBackupReason("hrneo-structured"))
	if err != nil {
		s.writeAudit(r, "config.write.structured", "hrneo", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.HRNeoConf, []byte(renderHRNeoConfigPreservingUnknown(merged, unknown)), 0o600); err != nil {
		s.writeAudit(r, "config.write.structured", "hrneo", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.HRNeoConf))

	resp := openapi.PutConfigResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.PutConfigResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "config.write.structured", "hrneo", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "config.write.structured", "hrneo", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}
