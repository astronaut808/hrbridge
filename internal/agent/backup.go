package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type BackupInfo = openapi.BackupInfo

func (s *Server) createBackup(reason string) (BackupInfo, error) {
	now := time.Now().UTC()
	id := now.Format("20060102T150405.000000000Z")
	if reason != "" {
		reason = sanitizeName(reason)
		if reason != "" {
			id += "-" + reason
		}
	}
	dir := filepath.Join(s.cfg.BackupDir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return BackupInfo{}, err
	}

	sources := map[string]string{
		"hrneo.conf":  s.cfg.HRNeoConf,
		"domain.conf": s.cfg.DomainConf,
		"ip.list":     s.cfg.CIDRList,
	}

	info := BackupInfo{Id: id, Created: now}
	for name, src := range sources {
		data, err := os.ReadFile(src) // #nosec G304 -- sources are fixed configured HR Neo files
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return BackupInfo{}, err
		}
		if err := atomicWriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return BackupInfo{}, err
		}
		info.Files = append(info.Files, name)
	}
	sort.Strings(info.Files)
	return info, nil
}

func (s *Server) listBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.cfg.BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(s.cfg.BackupDir, entry.Name())
		files, _ := os.ReadDir(dir)
		info := BackupInfo{Id: entry.Name()}
		if st, err := entry.Info(); err == nil {
			info.Created = st.ModTime().UTC()
		}
		for _, f := range files {
			if !f.IsDir() {
				info.Files = append(info.Files, f.Name())
			}
		}
		sort.Strings(info.Files)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id > out[j].Id })
	return out, nil
}

func (s *Server) restoreBackup(id string) ([]string, error) {
	if id != sanitizeBackupID(id) {
		return nil, fmt.Errorf("invalid backup id")
	}
	dir := filepath.Join(s.cfg.BackupDir, id)
	// codeql[go/path-injection] id must round-trip through sanitizeBackupID,
	// so the backup path stays below the configured backup directory.
	if st, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !st.IsDir() {
		return nil, fmt.Errorf("backup is not a directory")
	}

	targets := map[string]string{
		"hrneo.conf":  s.cfg.HRNeoConf,
		"domain.conf": s.cfg.DomainConf,
		"ip.list":     s.cfg.CIDRList,
	}
	var restored []string
	for name, target := range targets {
		src := filepath.Join(dir, name)
		// codeql[go/path-injection] Backup id is sanitized and filename comes
		// from the fixed targets allowlist above.
		data, err := os.ReadFile(src) // #nosec G304 -- backup ID is sanitized and filename comes from a fixed allowlist
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if err := atomicWriteFile(target, data, 0o600); err != nil {
			return nil, err
		}
		restored = append(restored, name)
	}
	sort.Strings(restored)
	return restored, nil
}

func sanitizeName(v string) string {
	v = strings.ToLower(v)
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteByte('-')
		}
	}
	if b.Len() > 48 {
		return b.String()[:48]
	}
	return b.String()
}

func sanitizeBackupID(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.' || r == 'T' || r == 'Z':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func configBackupReason(name string) string {
	return fmt.Sprintf("before-%s-write", name)
}
