package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestParseDomainConfigFile(t *testing.T) {
	path := t.TempDir() + "/domain.conf"
	content := strings.Join([]string{
		"##Google",
		"geosite:google,youtube.com,GoogleVideo.com/HydraRoute",
		"bad/token.example/OtherPolicy",
		"",
		"##Disabled block",
		"#/OldPolicy",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseDomainConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(cfg.Targets))
	}
	first := cfg.Targets[0]
	if first.Name != "HydraRoute" || !first.Enabled {
		t.Fatalf("unexpected first target: %#v", first)
	}
	if first.Comment == nil || *first.Comment != "Google" {
		t.Fatalf("unexpected comment: %#v", first.Comment)
	}
	if strings.Join(first.Geosite, ",") != "google" {
		t.Fatalf("unexpected geosite: %#v", first.Geosite)
	}
	if strings.Join(first.Domains, ",") != "youtube.com,googlevideo.com" {
		t.Fatalf("unexpected domains: %#v", first.Domains)
	}
	if cfg.Targets[1].Name != "OtherPolicy" || strings.Join(cfg.Targets[1].Domains, ",") != "bad/token.example" {
		t.Fatalf("expected parser to split by last slash like HR Neo, got %#v", cfg.Targets[1])
	}
	if cfg.Targets[2].Name != "OldPolicy" || cfg.Targets[2].Enabled {
		t.Fatalf("unexpected disabled target: %#v", cfg.Targets[2])
	}
}

func TestRenderDomainConfig(t *testing.T) {
	comment := "Google"
	cfg := openapi.DomainConfig{Targets: []openapi.DomainTarget{
		{
			Name:    "HydraRoute",
			Enabled: true,
			Comment: &comment,
			Domains: []string{"YouTube.com"},
			Geosite: []string{"google"},
		},
		{
			Name:    "OldPolicy",
			Enabled: false,
			Domains: []string{"ignored.example"},
			Geosite: []string{},
		},
	}}
	got := renderDomainConfig(cfg)
	for _, want := range []string{
		"##Google\n",
		"geosite:google,youtube.com/HydraRoute\n",
		"#/OldPolicy\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered config missing %q in:\n%s", want, got)
		}
	}
}

func TestDomainsStructuredEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/config/domains/structured", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	body, _ := json.Marshal(openapi.PutDomainConfigRequest{
		Config: openapi.DomainConfig{Targets: []openapi.DomainTarget{
			{Name: "HydraRoute", Enabled: true, Domains: []string{"example.com"}, Geosite: []string{}},
		}},
	})
	rr = perform(srv, "PUT", "/api/v1/config/domains/structured", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
