package agent

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
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
		req, err := decodeDomainRulePatchRequest(r, add)
		if err != nil {
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
			var duplicate duplicateRuleGroupError
			if errors.As(err, &duplicate) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
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
		req, err := decodeCIDRRulePatchRequest(r, add)
		if err != nil {
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
			var duplicate duplicateRuleGroupError
			if errors.As(err, &duplicate) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writePatchedConfig(w, r, "cidr-patch", s.cfg.CIDRList, content, req.Apply)
	}
}

func decodeDomainRulePatchRequest(r *http.Request, add bool) (openapi.DomainRulePatchRequest, error) {
	var req openapi.DomainRulePatchRequest
	if !add && r.URL.Query().Has("kind") {
		apply, err := optionalBoolQuery(r, "apply")
		if err != nil {
			return req, err
		}
		req.Kind = openapi.DomainRulePatchRequestKind(r.URL.Query().Get("kind"))
		req.Value = r.URL.Query().Get("value")
		req.Apply = apply
		if r.URL.Query().Has("comment") {
			comment := r.URL.Query().Get("comment")
			req.Comment = &comment
		}
		return req, nil
	}
	return req, decodeJSONBody(r, &req)
}

func decodeCIDRRulePatchRequest(r *http.Request, add bool) (openapi.CIDRRulePatchRequest, error) {
	var req openapi.CIDRRulePatchRequest
	if !add && r.URL.Query().Has("kind") {
		apply, err := optionalBoolQuery(r, "apply")
		if err != nil {
			return req, err
		}
		req.Kind = openapi.CIDRRulePatchRequestKind(r.URL.Query().Get("kind"))
		req.Value = r.URL.Query().Get("value")
		req.Apply = apply
		if r.URL.Query().Has("comment") {
			comment := r.URL.Query().Get("comment")
			req.Comment = &comment
		}
		return req, nil
	}
	return req, decodeJSONBody(r, &req)
}

func decodeJSONBody(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	return nil
}

func optionalBoolQuery(r *http.Request, name string) (*bool, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, errors.New(name + " must be a boolean")
	}
	return &parsed, nil
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
