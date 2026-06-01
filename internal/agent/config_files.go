package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) configPath(name string) (string, bool) {
	switch name {
	case "hrneo":
		return s.cfg.HRNeoConf, true
	case "domains":
		return s.cfg.DomainConf, true
	case "cidr":
		return s.cfg.CIDRList, true
	default:
		return "", false
	}
}

func (s *Server) handleGetConfig(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path, ok := s.configPath(name)
		if !ok {
			writeError(w, http.StatusNotFound, "unknown config")
			return
		}
		data, err := os.ReadFile(path) // #nosec G304 -- path is selected from a fixed config-file allowlist
		if err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		action := openapi.ConfigFileResponseRequiredAction(requiredActionForConfig(name))
		setRevisionHeader(w, fileRevision(path))
		writeJSON(w, http.StatusOK, openapi.ConfigFileResponse{
			Name:           openapi.ConfigFileResponseName(name),
			Path:           path,
			Content:        string(data),
			Exists:         err == nil,
			RequiredAction: &action,
		})
	}
}

func (s *Server) handlePutConfig(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path, ok := s.configPath(name)
		if !ok {
			writeError(w, http.StatusNotFound, "unknown config")
			return
		}

		req, err := decodePutConfig(r)
		if err != nil {
			s.writeAudit(r, "config.write", name, false, "", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !checkRevisionPrecondition(r, path) {
			writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
			return
		}

		backup, err := s.createBackup(configBackupReason(name))
		if err != nil {
			s.writeAudit(r, "config.write", name, false, "", err)
			writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
			return
		}
		if err := atomicWriteFile(path, []byte(req.Content), 0o600); err != nil {
			s.writeAudit(r, "config.write", name, false, backup.Id, err)
			writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
			return
		}
		setRevisionHeader(w, fileRevision(path))

		action := openapi.PutConfigResponseRequiredAction(requiredActionForConfig(name))
		resp := openapi.PutConfigResponse{
			Saved:          true,
			Backup:         backup,
			RequiredAction: action,
		}
		if req.Apply != nil && *req.Apply {
			out, err := s.runService(string(resp.RequiredAction))
			resp.ApplyOutput = &out
			if err != nil {
				s.writeAudit(r, "config.write", name, false, backup.Id, err)
				writeJSON(w, http.StatusAccepted, resp)
				return
			}
			resp.Applied = true
		}
		s.writeAudit(r, "config.write", name, true, backup.Id, nil)
		writeJSON(w, http.StatusOK, resp)
	}
}

func decodePutConfig(r *http.Request) (openapi.PutConfigRequest, error) {
	defer func() { _ = r.Body.Close() }()
	if ct := r.Header.Get("Content-Type"); ct == "text/plain" {
		data, err := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
		if err != nil {
			return openapi.PutConfigRequest{}, err
		}
		if len(data) > 4<<20 {
			return openapi.PutConfigRequest{}, fmt.Errorf("config exceeds %d bytes", 4<<20)
		}
		return openapi.PutConfigRequest{Content: string(data)}, nil
	}
	var req openapi.PutConfigRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func requiredActionForConfig(name string) string {
	switch name {
	case "hrneo", "domains", "cidr":
		return "restart"
	default:
		return "none"
	}
}
