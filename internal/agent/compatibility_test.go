package agent

import (
	"bytes"
	"net/http"
	"os"
	"testing"
)

func TestCompatibilityEndpointReportsSupportedVersion(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/compatibility", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"supportedHrneoVersion":"3.11.0-1"`),
		[]byte(`"code":"supported-hrneo-version"`),
		[]byte(`"code":"hrneo-config-readable"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestCompatibilityEndpointReportsConfigIssues(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("autoStart=true\nFutureKey=value\nGeoIPFile=/no/such/geoip.dat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/HydraRoute\n1.1.1.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/compatibility", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":false`),
		[]byte(`"code":"unknown-hrneo-key"`),
		[]byte(`"code":"missing-hrneo-key"`),
		[]byte(`"code":"geodata-file-missing"`),
		[]byte(`"code":"invalid-cidr-config"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestCompatibilityAcceptsHRNeoDirectiveComments(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.DomainConf, []byte("##geosite: TELEGRAM / geoip: TELEGRAM\ngeosite:telegram/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("##impossible to use\n#/Too-big-geoip-tag\ngeoip:telegram\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/compatibility", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, unwanted := range [][]byte{
		[]byte(`"code":"invalid-domain-config"`),
		[]byte(`"code":"invalid-cidr-config"`),
	} {
		if bytes.Contains(rr.Body.Bytes(), unwanted) {
			t.Fatalf("unexpected %s in %s", unwanted, rr.Body.String())
		}
	}
}
