package agent

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoDataReferencesAndValidation(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.DomainConf, []byte("geosite:youtube/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/HydraRoute\ngeoip:RU\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/geodata/references", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("references status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{[]byte(`"geoip":["ru"]`), []byte(`"geosite":["youtube"]`)} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}

	rr = perform(srv, "POST", "/api/v1/geodata/validate", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":false`),
		[]byte(`"code":"geoip-file-missing"`),
		[]byte(`"code":"geosite-file-missing"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}

	rr = perform(srv, "GET", "/api/v1/geodata/validate", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("validate get status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGeoDataValidationUsesExactIndexAndReportsAutoDisabledTags(t *testing.T) {
	srv, cfg := testServer(t)
	geoip := filepath.Join(t.TempDir(), "geoip.dat")
	geosite := filepath.Join(t.TempDir(), "geosite.dat")
	if err := os.WriteFile(geoip, geoDataDAT("RU", "US"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(geosite, geoDataDAT("YOUTUBE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.HRNeoConf, []byte("GeoIPFile="+geoip+"\nGeoSiteFile="+geosite+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DomainConf, []byte("geosite:missing/HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/HydraRoute\ngeoip:RU\n\n##impossible to use\n#/Too-big-geoip-tag\ngeoip:telegram\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/geodata/validate", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"ok":true`),
		[]byte(`"code":"geosite-tag-not-found"`),
		[]byte(`"code":"geoip-tag-auto-disabled"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
	if bytes.Contains(rr.Body.Bytes(), []byte(`"code":"geoip-tag-not-found"`)) {
		t.Fatalf("configured geoip tag reported missing: %s", rr.Body.String())
	}
}
