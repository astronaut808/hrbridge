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

	rr = perform(srv, "DELETE", "/api/v1/config/domains/targets/HydraRoute/rules?kind=domain&value=new.example", "test-token", nil)
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

	rr = perform(srv, "DELETE", "/api/v1/config/cidr/targets/HydraRoute/rules?kind=cidr&value=8.8.8.8%2F32", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("cidr delete status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPatchDomainRuleCanTargetCommentGroup(t *testing.T) {
	srv, cfg := testServer(t)
	comment := "Music"
	body, _ := json.Marshal(openapi.DomainRulePatchRequest{
		Kind:    openapi.DomainRulePatchRequestKindDomain,
		Value:   "spotify.com",
		Comment: &comment,
	})
	rr := perform(srv, "POST", "/api/v1/config/domains/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("grouped domain add status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "##Music\nspotify.com/HydraRoute") {
		t.Fatalf("grouped domain patch not persisted: %q", string(data))
	}

	rr = perform(srv, "DELETE", "/api/v1/config/domains/targets/HydraRoute/rules?kind=domain&value=spotify.com&comment=Music", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("grouped domain delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err = os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "spotify.com") || strings.Contains(string(data), "##Music") {
		t.Fatalf("grouped domain patch left stale data: %q", string(data))
	}
}

func TestPatchDomainRuleRejectsDuplicateInAnotherCommentGroup(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.DomainConf, []byte("##AI\nopenai.com/HydraRoute\n\n##Music\nsoundcloud.com/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	comment := "Music"
	body, _ := json.Marshal(openapi.DomainRulePatchRequest{
		Kind:    openapi.DomainRulePatchRequestKindDomain,
		Value:   "openai.com",
		Comment: &comment,
	})
	rr := perform(srv, "POST", "/api/v1/config/domains/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate grouped domain status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `group \"AI\"`) {
		t.Fatalf("unexpected duplicate response: %s", rr.Body.String())
	}
}

func TestPatchCIDRRuleRejectsDuplicateInAnotherCommentGroup(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.CIDRList, []byte("##Cloudflare\n/HydraRoute\n1.1.1.1/32\n\n##Telegram\n/HydraRoute\ngeoip:telegram\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	comment := "Telegram"
	body, _ := json.Marshal(openapi.CIDRRulePatchRequest{
		Kind:    openapi.CIDRRulePatchRequestKindCidr,
		Value:   "1.1.1.1/32",
		Comment: &comment,
	})
	rr := perform(srv, "POST", "/api/v1/config/cidr/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate grouped CIDR status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `group \"Cloudflare\"`) {
		t.Fatalf("unexpected duplicate response: %s", rr.Body.String())
	}
}

func TestDeleteRuleAcceptsLegacyJSONBody(t *testing.T) {
	srv, _ := testServer(t)

	body, _ := json.Marshal(openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "example.com"})
	rr := perform(srv, "DELETE", "/api/v1/config/domains/targets/HydraRoute/rules", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("legacy domain delete status=%d body=%s", rr.Code, rr.Body.String())
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
