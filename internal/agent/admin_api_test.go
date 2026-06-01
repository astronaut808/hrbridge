package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestDoctorAndOverview(t *testing.T) {
	srv, _ := testServer(t)

	rr := perform(srv, "GET", "/api/v1/doctor", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("doctor status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"checks"`)) {
		t.Fatalf("doctor missing checks: %s", rr.Body.String())
	}

	rr = perform(srv, "GET", "/api/v1/overview", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("overview status=%d body=%s", rr.Code, rr.Body.String())
	}
	var overview openapi.OverviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.TargetCount != 1 || overview.DomainRuleCount != 1 || overview.CidrRuleCount != 1 {
		t.Fatalf("unexpected overview: %#v", overview)
	}
}

func TestConfigRevisionPrecondition(t *testing.T) {
	srv, _ := testServer(t)

	rr := perform(srv, "GET", "/api/v1/config/domains", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("ETag") == "" || rr.Header().Get("X-Config-Revision") == "" {
		t.Fatalf("missing revision headers: %#v", rr.Header())
	}

	body, _ := json.Marshal(openapi.PutConfigRequest{Content: "openai.com/HydraRoute\n"})
	rr = performWithHeaders(srv, "PUT", "/api/v1/config/domains", "test-token", body, map[string]string{"If-Match": "bad-revision"})
	if rr.Code != http.StatusPreconditionFailed {
		t.Fatalf("put status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = perform(srv, "PUT", "/api/v1/config/domains", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("ETag") == "" || rr.Header().Get("X-Config-Revision") == "" {
		t.Fatalf("missing write revision headers: %#v", rr.Header())
	}
}

func TestPatchDomainAndCIDRRules(t *testing.T) {
	srv, cfg := testServer(t)

	body, _ := json.Marshal(openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "New.Example"})
	rr := perform(srv, "POST", "/api/v1/config/domains/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("domain add status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "new.example/HydraRoute") {
		t.Fatalf("domain patch not persisted: %q", string(data))
	}

	body, _ = json.Marshal(openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "new.example"})
	rr = perform(srv, "DELETE", "/api/v1/config/domains/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("domain delete status=%d body=%s", rr.Code, rr.Body.String())
	}

	body, _ = json.Marshal(openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "8.8.8.8/32"})
	rr = perform(srv, "POST", "/api/v1/config/cidr/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("cidr add status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err = os.ReadFile(cfg.CIDRList)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "8.8.8.8/32") {
		t.Fatalf("cidr patch not persisted: %q", string(data))
	}

	body, _ = json.Marshal(openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "8.8.8.8/32"})
	rr = perform(srv, "DELETE", "/api/v1/config/cidr/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("cidr delete status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRotateTokenPersistsNewToken(t *testing.T) {
	srv, cfg := testServer(t)

	rr := perform(srv, "POST", "/api/v1/auth/token/rotate", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("rotate status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp openapi.TokenRotateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" || resp.Token == "test-token" {
		t.Fatalf("unexpected token: %q", resp.Token)
	}
	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "authToken="+resp.Token) {
		t.Fatalf("config does not contain rotated token: %s", string(data))
	}
}

func TestGeoDataTagsExactIndex(t *testing.T) {
	srv, cfg := testServer(t)
	geoip := filepath.Join(t.TempDir(), "geoip.dat")
	geosite := filepath.Join(t.TempDir(), "geosite.dat")
	if err := os.WriteFile(geoip, geoDataDAT("RU", "US", "PRIVATE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(geosite, geoDataDAT("YOUTUBE", "CATEGORY-ADS", "GOOGLE_CN"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.HRNeoConf, []byte("GeoIPFile="+geoip+"\nGeoSiteFile="+geosite+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/geodata/tags", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("tags status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp openapi.GeoDataTagsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !containsString(resp.Geoip, "RU") || !containsString(resp.Geoip, "US") {
		t.Fatalf("missing geoip tags: %#v", resp.Geoip)
	}
	if !containsString(resp.Geosite, "youtube") || !containsString(resp.Geosite, "category-ads") || !containsString(resp.Geosite, "google_cn") {
		t.Fatalf("missing geosite tags: %#v", resp.Geosite)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
