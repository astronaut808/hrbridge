package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestParseCIDRConfigFile(t *testing.T) {
	path := t.TempDir() + "/ip.list"
	content := strings.Join([]string{
		"##Cloudflare",
		"/HydraRoute",
		"geoip:cloudflare",
		"1.1.1.1/32",
		"2606:4700::/32",
		"",
		"##Disabled ranges",
		"#/OldPolicy",
		"10.0.0.0/8",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseCIDRConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(cfg.Blocks))
	}
	first := cfg.Blocks[0]
	if first.Target != "HydraRoute" || !first.Enabled {
		t.Fatalf("unexpected first block: %#v", first)
	}
	if first.Comment == nil || *first.Comment != "Cloudflare" {
		t.Fatalf("unexpected comment: %#v", first.Comment)
	}
	if strings.Join(first.Geoip, ",") != "cloudflare" {
		t.Fatalf("unexpected geoip: %#v", first.Geoip)
	}
	if strings.Join(first.Entries, ",") != "1.1.1.1/32,2606:4700::/32" {
		t.Fatalf("unexpected entries: %#v", first.Entries)
	}
	if cfg.Blocks[1].Target != "OldPolicy" || cfg.Blocks[1].Enabled {
		t.Fatalf("unexpected disabled block: %#v", cfg.Blocks[1])
	}
}

func TestRenderCIDRConfig(t *testing.T) {
	comment := "Cloudflare"
	cfg := openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
		{
			Target:  "HydraRoute",
			Enabled: true,
			Comment: &comment,
			Geoip:   []string{"cloudflare"},
			Entries: []string{"1.1.1.1/32"},
		},
		{
			Target:  "OldPolicy",
			Enabled: false,
			Geoip:   []string{},
			Entries: []string{"10.0.0.0/8"},
		},
	}}
	got := renderCIDRConfig(cfg)
	for _, want := range []string{
		"##Cloudflare\n",
		"/HydraRoute\n",
		"geoip:cloudflare\n",
		"1.1.1.1/32\n",
		"#/OldPolicy\n",
		"10.0.0.0/8\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered config missing %q in:\n%s", want, got)
		}
	}
}

func TestCIDRStructuredEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/config/cidr/structured", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	body, _ := json.Marshal(openapi.PutCIDRConfigRequest{
		Config: openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
			{
				Target:  "HydraRoute",
				Enabled: true,
				Geoip:   []string{"cloudflare"},
				Entries: []string{"1.1.1.1/32"},
			},
		}},
	})
	rr = perform(srv, "PUT", "/api/v1/config/cidr/structured", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
