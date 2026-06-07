package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

var errMissingBackupID = errors.New("missing backup id")

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openapi.HealthResponse{
		Ok:      true,
		Service: openapi.HealthResponseService("HydraBridge"),
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openapi.VersionResponse{
		AgentVersion: Version,
		ApiVersion:   "v1",
		Capabilities: []string{
			"config.raw",
			"config.hrneo.defaults",
			"config.hrneo.metadata",
			"service.control",
			"backup.create",
			"audit.read",
			"auth.token.rotate",
			"compatibility.read",
			"diagnostics.domain",
			"diagnostics.ip",
			"diagnostics.ip.runtimeEvidence",
			"doctor.read",
			"geodata.tags",
			"geodata.references",
			"geodata.validate",
			"import.text",
			"logs.read",
			"overview.read",
			"config.revision",
			"config.patch.rules",
			"config.patch.ruleGroups",
			"views.grouped",
			"runtime.firewall",
			"runtime.ipsets",
			"runtime.policies",
			"runtime.directRoutes",
			"status.hrneo",
		},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := openapi.StatusResponse{
		Hrneo: s.hrneoStatus(),
	}
	resp.Hrbridge.Version = Version
	resp.Hrbridge.UptimeSec = int64(sinceSeconds(s.started))
	resp.Paths.HrneoConf = s.cfg.HRNeoConf
	resp.Paths.DomainConf = s.cfg.DomainConf
	resp.Paths.CidrList = s.cfg.CIDRList
	resp.Paths.BackupDir = s.cfg.BackupDir
	resp.Paths.AuditLog = s.cfg.AuditLog
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := s.listBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	backup, err := s.createBackup("manual")
	if err != nil {
		s.writeAudit(r, "backup.create", "", false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r, "backup.create", "", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, backup)
}

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.RestoreBackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "backup.restore", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Id == "" {
		s.writeAudit(r, "backup.restore", "", false, "", errMissingBackupID)
		writeError(w, http.StatusBadRequest, "missing backup id")
		return
	}

	safety, err := s.createBackup("before-restore")
	if err != nil {
		s.writeAudit(r, "backup.restore", "", false, "", err)
		writeError(w, http.StatusInternalServerError, "safety backup failed: "+err.Error())
		return
	}
	restored, err := s.restoreBackup(req.Id)
	if err != nil {
		s.writeAudit(r, "backup.restore", "", false, safety.Id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := openapi.RestoreBackupResponse{
		Restored:       restored,
		SafetyBackup:   safety,
		RequiredAction: openapi.RestoreBackupResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		resp.Applied = err == nil
		if err != nil {
			s.writeAudit(r, "backup.restore", "", false, safety.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
	}
	s.writeAudit(r, "backup.restore", "", true, safety.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 300
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}
	lines, err := tailFile(s.cfg.LogFile, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.LogsResponse{
		Path:  s.cfg.LogFile,
		Lines: lines,
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	events, err := s.readAudit(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.AuditEventsResponse{
		Path:   s.cfg.AuditLog,
		Events: events,
	})
}
