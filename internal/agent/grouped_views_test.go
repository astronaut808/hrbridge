package agent

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestGroupDomainTargetsAggregatesWithoutDroppingGroups(t *testing.T) {
	cfg := openapi.DomainConfig{Targets: []openapi.DomainTarget{
		{Name: "Finland", Enabled: true, Domains: []string{"fbcdn.net"}, Geosite: []string{}},
		{Name: "Finland", Enabled: true, Domains: []string{"FBCDN.NET", "instagram.com"}, Geosite: []string{"telegram"}},
		{Name: "Russia", Enabled: true, Domains: []string{"youtube.com"}, Geosite: []string{}},
	}}
	got := groupDomainTargets(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %#v", got)
	}
	if got[0].Name != "Finland" || len(got[0].Groups) != 2 {
		t.Fatalf("unexpected Finland view: %#v", got[0])
	}
	if len(got[0].Domains) != 2 || got[0].Domains[0] != "fbcdn.net" || got[0].Domains[1] != "instagram.com" {
		t.Fatalf("unexpected Finland domains: %#v", got[0].Domains)
	}
}

func TestGroupedViewsEndpoints(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.DomainConf, []byte("fbcdn.net/Finland\ninstagram.com,fbcdn.net/Finland\nyoutube.com/Russia\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/views/domains/grouped", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("domains status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{[]byte(`"name":"Finland"`), []byte(`"domains":["fbcdn.net","instagram.com"]`), []byte(`"groups":[`)} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}

	rr = perform(srv, "GET", "/api/v1/views/cidr/grouped", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("cidr status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"target":"HydraRoute"`)) {
		t.Fatalf("unexpected CIDR body=%s", rr.Body.String())
	}
}
