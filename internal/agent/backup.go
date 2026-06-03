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
	dir, err := s.validatedBackupDir(id)
	if err != nil {
		return nil, err
	}
	// Backup id has passed strict character validation and a containment check
	// against the configured backup directory.
	// lgtm[go/path-injection]
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
		src, err := backupFilePath(dir, name)
		if err != nil {
			return nil, err
		}
		// Backup id is constrained to the backup directory, and backup file
		// name is selected from the fixed HR Neo file allowlist.
		// lgtm[go/path-injection]
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

func (s *Server) validatedBackupDir(id string) (string, error) {
	if err := validateBackupID(id); err != nil {
		return "", err
	}
	base, err := filepath.Abs(s.cfg.BackupDir)
	if err != nil {
		return "", err
	}
	dir, err := filepath.Abs(filepath.Join(base, id))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup id escapes backup directory")
	}
	return dir, nil
}

func validateBackupID(id string) error {
	if id == "" {
		return fmt.Errorf("invalid backup id")
	}
	if len(id) > 128 {
		return fmt.Errorf("invalid backup id")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("invalid backup id")
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return fmt.Errorf("invalid backup id")
		}
	}
	return nil
}

func backupFilePath(dir, name string) (string, error) {
	switch name {
	case "hrneo.conf", "domain.conf", "ip.list":
	default:
		return "", fmt.Errorf("invalid backup file")
	}
	if filepath.Base(name) != name {
		return "", fmt.Errorf("invalid backup file")
	}
	return filepath.Join(dir, name), nil
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

func configBackupReason(name string) string {
	return fmt.Sprintf("before-%s-write", name)
}
