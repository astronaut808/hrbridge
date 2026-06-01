package agent

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) writeAudit(r *http.Request, action, target string, ok bool, backupID string, err error) {
	if strings.TrimSpace(s.cfg.AuditLog) == "" {
		return
	}
	event := openapi.AuditEvent{
		Action: action,
		Ok:     ok,
		Time:   time.Now().UTC(),
	}
	if target != "" {
		event.Target = &target
	}
	if backupID != "" {
		event.BackupId = &backupID
	}
	if err != nil {
		msg := err.Error()
		event.Error = &msg
	}
	if r != nil {
		remote := remoteAddr(r)
		if remote != "" {
			event.Remote = &remote
		}
	}

	data, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.AuditLog), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(s.cfg.AuditLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(data, '\n'))
}

func (s *Server) readAudit(limit int) ([]openapi.AuditEvent, error) {
	lines, err := tailFile(s.cfg.AuditLog, limit)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	events := make([]openapi.AuditEvent, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event openapi.AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}
	return events, nil
}

func remoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
