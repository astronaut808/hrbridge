package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestStructuredEndpointsRejectInvalidPayloads(t *testing.T) {
	srv, _ := testServer(t)

	badDomain, _ := json.Marshal(openapi.PutDomainConfigRequest{
		Config: openapi.DomainConfig{Targets: []openapi.DomainTarget{
			{Name: "", Enabled: true, Domains: []string{"example.com"}, Geosite: []string{}},
		}},
	})
	rr := perform(srv, "PUT", "/api/v1/config/domains/structured", "test-token", badDomain)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("domain status=%d body=%s", rr.Code, rr.Body.String())
	}

	badCIDR, _ := json.Marshal(openapi.PutCIDRConfigRequest{
		Config: openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
			{Target: "HydraRoute", Enabled: true, Entries: []string{"not-a-prefix"}, Geoip: []string{}},
		}},
	})
	rr = perform(srv, "PUT", "/api/v1/config/cidr/structured", "test-token", badCIDR)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("cidr status=%d body=%s", rr.Code, rr.Body.String())
	}

	timeout := -1
	logMode := openapi.HRNeoConfigLog("verbose")
	badHRNeo, _ := json.Marshal(openapi.PutHRNeoConfigRequest{
		Config: openapi.HRNeoConfig{
			IpsetTimeout: &timeout,
			Log:          &logMode,
		},
	})
	rr = perform(srv, "PUT", "/api/v1/config/hrneo/structured", "test-token", badHRNeo)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("hrneo status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDomainValidationReportsConflicts(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutDomainConfigRequest{
		Config: openapi.DomainConfig{Targets: []openapi.DomainTarget{
			{Name: "HydraRoute", Enabled: true, Domains: []string{"example.com"}, Geosite: []string{"google"}},
			{Name: "OtherPolicy", Enabled: true, Domains: []string{"Example.com"}, Geosite: []string{"google"}},
		}},
	})
	rr := perform(srv, "POST", "/api/v1/config/domains/validate", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":true`),
		[]byte(`"code":"conflicting-domain"`),
		[]byte(`"code":"conflicting-geosite"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestDomainValidationDeduplicatesRepeatedWarnings(t *testing.T) {
	issues := validateDomainConfigDeep(openapi.DomainConfig{Targets: []openapi.DomainTarget{
		{Name: "Finland", Enabled: true, Domains: []string{"fbcdn.net"}, Geosite: []string{"meta"}},
		{Name: "Finland", Enabled: true, Domains: []string{"fbcdn.net", "fbcdn.net"}, Geosite: []string{"meta"}},
	}})
	var domains, geosite int
	for _, issue := range issues {
		switch issue.Code {
		case "duplicate-domain":
			domains++
		case "duplicate-geosite":
			geosite++
		}
	}
	if domains != 1 || geosite != 1 {
		t.Fatalf("expected one warning per repeated rule, got domain=%d geosite=%d issues=%#v", domains, geosite, issues)
	}
}

func TestCIDRValidationReportsConflictingOverlap(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutCIDRConfigRequest{
		Config: openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
			{Target: "HydraRoute", Enabled: true, Entries: []string{"10.0.0.0/8"}, Geoip: []string{}},
			{Target: "OtherPolicy", Enabled: true, Entries: []string{"10.1.0.0/16"}, Geoip: []string{}},
		}},
	})
	rr := perform(srv, "POST", "/api/v1/config/cidr/validate", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":true`),
		[]byte(`"code":"conflicting-cidr"`),
		[]byte(`"severity":"warning"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestCIDRValidationReportsDuplicateAsWarning(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutCIDRConfigRequest{
		Config: openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
			{Target: "HydraRoute", Enabled: true, Entries: []string{"1.1.1.1/32", "1.1.1.1/32"}, Geoip: []string{}},
		}},
	})
	rr := perform(srv, "POST", "/api/v1/config/cidr/validate", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":true`),
		[]byte(`"code":"duplicate-cidr"`),
		[]byte(`"severity":"warning"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestCIDRValidationRejectsPlainIP(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutCIDRConfigRequest{
		Config: openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
			{Target: "HydraRoute", Enabled: true, Entries: []string{"1.1.1.1"}, Geoip: []string{}},
		}},
	})
	rr := perform(srv, "POST", "/api/v1/config/cidr/validate", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":false`),
		[]byte(`"code":"invalid-cidr-config"`),
		[]byte(`explicit mask`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestValidationAcceptsHRNeoCommentsContainingDirectives(t *testing.T) {
	domainComment := "geosite: TELEGRAM / geoip: TELEGRAM"
	domainIssues := validateDomainConfigDeep(openapi.DomainConfig{Targets: []openapi.DomainTarget{
		{Name: "Finland", Enabled: true, Domains: []string{}, Geosite: []string{"telegram"}, Comment: &domainComment},
	}})
	if len(domainIssues) != 0 {
		t.Fatalf("unexpected domain issues: %#v", domainIssues)
	}

	cidrComment := "impossible to use / Too-big-geoip-tag"
	cidrIssues := validateCIDRConfigDeep(openapi.CIDRConfig{Blocks: []openapi.CIDRBlock{
		{Target: "Too-big-geoip-tag", Enabled: false, Entries: []string{}, Geoip: []string{"telegram"}, Comment: &cidrComment},
	}})
	if len(cidrIssues) != 0 {
		t.Fatalf("unexpected CIDR issues: %#v", cidrIssues)
	}
}
