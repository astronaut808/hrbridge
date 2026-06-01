package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func testServer(t *testing.T) (*http.Server, Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Listen = "127.0.0.1:0"
	cfg.AuthToken = "test-token"
	cfg.ConfigPath = filepath.Join(dir, "hrbridge.conf")
	cfg.BackupDir = filepath.Join(dir, "backups")
	cfg.HRNeoConf = filepath.Join(dir, "hrneo.conf")
	cfg.DomainConf = filepath.Join(dir, "domain.conf")
	cfg.CIDRList = filepath.Join(dir, "ip.list")
	cfg.LogFile = filepath.Join(dir, "hrneo.log")
	cfg.AuditLog = filepath.Join(dir, "audit.log")
	cfg.HRNeoPID = filepath.Join(dir, "hrneo.pid")
	cfg.HRNeoInit = filepath.Join(dir, "S99hrneo")
	cfg.RCIURL = "http://127.0.0.1:1"
	if err := os.WriteFile(cfg.HRNeoConf, []byte("autoStart=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DomainConf, []byte("example.com/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/HydraRoute\n1.1.1.1/32\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.LogFile, []byte("[INFO] one\n[WARN] two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewServer(cfg), cfg
}

func perform(srv *http.Server, method, path, token string, body []byte) *httptest.ResponseRecorder {
	return performWithHeaders(srv, method, path, token, body, nil)
}

func performWithHeaders(srv *http.Server, method, path, token string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, req)
	return rr
}

func TestHealthIsPublic(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/health", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthRequired(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/version", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCORSPreflight(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Listen = "127.0.0.1:0"
	cfg.AuthToken = "test-token"
	cfg.AllowOrigins = "http://router.local"
	cfg.BackupDir = filepath.Join(dir, "backups")
	cfg.HRNeoConf = filepath.Join(dir, "hrneo.conf")
	cfg.DomainConf = filepath.Join(dir, "domain.conf")
	cfg.CIDRList = filepath.Join(dir, "ip.list")
	cfg.LogFile = filepath.Join(dir, "hrneo.log")
	cfg.AuditLog = filepath.Join(dir, "audit.log")
	cfg.HRNeoPID = filepath.Join(dir, "hrneo.pid")
	cfg.HRNeoInit = filepath.Join(dir, "S99hrneo")
	cfg.RCIURL = "http://127.0.0.1:1"

	srv := NewServer(cfg)
	rr := perform(srv, "OPTIONS", "/api/v1/config/domains", "", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://router.local" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}

func TestConfigReadWriteCreatesBackup(t *testing.T) {
	srv, cfg := testServer(t)

	rr := perform(srv, "GET", "/api/v1/config/domains", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rr.Code, rr.Body.String())
	}

	body, _ := json.Marshal(openapi.PutConfigRequest{Content: "openai.com/nwg0\n"})
	rr = perform(srv, "PUT", "/api/v1/config/domains", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rr.Code, rr.Body.String())
	}

	data, err := os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "openai.com/nwg0\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}

	entries, err := os.ReadDir(cfg.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one backup, got %d", len(entries))
	}
}

func TestLogs(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/logs?limit=1", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("[WARN] two")) {
		t.Fatalf("unexpected body=%s", rr.Body.String())
	}
}

func TestAuditRecordsConfigWrites(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutConfigRequest{Content: "openai.com/nwg0\n"})
	rr := perform(srv, "PUT", "/api/v1/config/domains", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = perform(srv, "GET", "/api/v1/audit?limit=10", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"action":"config.write"`)) {
		t.Fatalf("audit missing config.write: %s", rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"target":"domains"`)) {
		t.Fatalf("audit missing target: %s", rr.Body.String())
	}
}
