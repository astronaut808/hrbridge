package agent

import (
	"os"
	"strings"
	"testing"
)

func TestRestoreBackupRestoresAllowedFiles(t *testing.T) {
	_, cfg := testServer(t)
	s := &Server{cfg: cfg}

	backup, err := s.createBackup("manual")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DomainConf, []byte("changed.example/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	restored, err := s.restoreBackup(backup.Id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(restored, ",") != "domain.conf,hrneo.conf,ip.list" {
		t.Fatalf("unexpected restored files: %v", restored)
	}
	data, err := os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "example.com/HydraRoute\n" {
		t.Fatalf("domain config was not restored: %q", string(data))
	}
}

func TestRestoreBackupRejectsUnsafeIDs(t *testing.T) {
	_, cfg := testServer(t)
	s := &Server{cfg: cfg}

	for _, id := range []string{
		"",
		"../outside",
		"nested/backup",
		`nested\backup`,
		"20260603T213847..498167894Z-manual",
		strings.Repeat("a", 129),
		"20260603T213847.498167894Z-manual!",
	} {
		t.Run(id, func(t *testing.T) {
			if _, err := s.restoreBackup(id); err == nil {
				t.Fatalf("restoreBackup(%q) succeeded", id)
			}
		})
	}
}
