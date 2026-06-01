package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestParseHRNeoConfigFile(t *testing.T) {
	path := t.TempDir() + "/hrneo.conf"
	content := strings.Join([]string{
		"autoStart=true",
		"CIDR=false",
		"IpsetTimeout=21600",
		"GeoIPFile=/opt/geoip_RU.dat",
		"GeoIPFile=/opt/geoip.dat",
		"PolicyOrder=HydraRoute,nwg0",
		"UnknownKey=value",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, unknown, err := parseHRNeoConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AutoStart == nil || !*cfg.AutoStart {
		t.Fatal("autoStart was not parsed")
	}
	if cfg.CIDR == nil || *cfg.CIDR {
		t.Fatal("CIDR was not parsed")
	}
	if cfg.IpsetTimeout == nil || *cfg.IpsetTimeout != 21600 {
		t.Fatalf("unexpected IpsetTimeout: %#v", cfg.IpsetTimeout)
	}
	if cfg.GeoIPFile == nil || len(*cfg.GeoIPFile) != 2 {
		t.Fatalf("unexpected GeoIPFile: %#v", cfg.GeoIPFile)
	}
	if cfg.PolicyOrder == nil || strings.Join(*cfg.PolicyOrder, ",") != "HydraRoute,nwg0" {
		t.Fatalf("unexpected PolicyOrder: %#v", cfg.PolicyOrder)
	}
	if unknown["UnknownKey"] != "value" {
		t.Fatalf("unexpected unknown map: %#v", unknown)
	}
}

func TestRenderHRNeoConfig(t *testing.T) {
	autoStart := true
	geo := []string{"/opt/geoip.dat"}
	order := []string{"HydraRoute", "nwg0"}
	cfg := openapi.HRNeoConfig{
		AutoStart:   &autoStart,
		GeoIPFile:   &geo,
		PolicyOrder: &order,
	}
	got := renderHRNeoConfig(cfg)
	for _, want := range []string{
		"autoStart=true\n",
		"GeoIPFile=/opt/geoip.dat\n",
		"PolicyOrder=HydraRoute,nwg0\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered config missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderHRNeoConfigPreservesUnknownKeys(t *testing.T) {
	autoStart := false
	cfg := openapi.HRNeoConfig{AutoStart: &autoStart}
	got := renderHRNeoConfigPreservingUnknown(cfg, map[string]string{"FutureKey": "enabled"})
	if !strings.Contains(got, "autoStart=false\n") {
		t.Fatalf("rendered config missing autoStart in:\n%s", got)
	}
	if !strings.Contains(got, "FutureKey=enabled\n") {
		t.Fatalf("rendered config did not preserve unknown key in:\n%s", got)
	}
}

func TestHRNeoStructuredEndpoint(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("autoStart=true\nCIDR=false\nUnknownKey=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rr := perform(srv, "GET", "/api/v1/config/hrneo/structured", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	autoStart := false
	body, _ := json.Marshal(openapi.PutHRNeoConfigRequest{
		Config: openapi.HRNeoConfig{AutoStart: &autoStart},
	})
	rr = perform(srv, "PUT", "/api/v1/config/hrneo/structured", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	saved, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"autoStart=false\n",
		"CIDR=false\n",
		"UnknownKey=value\n",
	} {
		if !strings.Contains(string(saved), want) {
			t.Fatalf("saved config missing %q in:\n%s", want, saved)
		}
	}
}
