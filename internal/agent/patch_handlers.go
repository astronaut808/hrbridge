package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

var errInvalidRuleKind = errors.New("invalid rule kind")

func (s *Server) handlePatchDomainRule(add bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		target := strings.TrimSpace(r.PathValue("target"))
		if err := validateTargetName(target); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !checkRevisionPrecondition(r, s.cfg.DomainConf) {
			writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
			return
		}
		var req openapi.DomainRulePatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		data, err := os.ReadFile(s.cfg.DomainConf)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		content, err := patchDomainConfigText(string(data), target, req, add)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writePatchedConfig(w, r, "domains-patch", s.cfg.DomainConf, content, req.Apply)
	}
}

func (s *Server) handlePatchCIDRRule(add bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		target := strings.TrimSpace(r.PathValue("target"))
		if err := validateTargetName(target); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !checkRevisionPrecondition(r, s.cfg.CIDRList) {
			writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
			return
		}
		var req openapi.CIDRRulePatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		data, err := os.ReadFile(s.cfg.CIDRList)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		content, err := patchCIDRConfigText(string(data), target, req, add)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writePatchedConfig(w, r, "cidr-patch", s.cfg.CIDRList, content, req.Apply)
	}
}

func (s *Server) writePatchedConfig(w http.ResponseWriter, r *http.Request, reason, path, content string, apply *bool) {
	backup, err := s.createBackup(configBackupReason(reason))
	if err != nil {
		s.writeAudit(r, "config.patch", reason, false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(path, []byte(content), 0o600); err != nil {
		s.writeAudit(r, "config.patch", reason, false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(path))
	resp := openapi.PutConfigResponse{Saved: true, Backup: backup, RequiredAction: openapi.PutConfigResponseRequiredActionRestart}
	if apply != nil && *apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "config.patch", reason, false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "config.patch", reason, true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}
