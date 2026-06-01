package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestExportCSVEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/export/csv", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("section,target,kind,value,enabled,comment")) {
		t.Fatalf("missing header: %s", rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("domain,HydraRoute,domain,example.com,true,")) {
		t.Fatalf("missing domain row: %s", rr.Body.String())
	}
}

func TestImportCSVPreviewEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.ImportCSVRequest{
		Content: "section,target,kind,value,enabled,comment\n" +
			"domain,HydraRoute,domain,example.com,true,\n" +
			"cidr,HydraRoute,cidr,1.1.1.1/32,true,\n",
	})
	rr := perform(srv, "POST", "/api/v1/import/csv/preview", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("unexpected preview: %s", rr.Body.String())
	}
}

func TestImportCSVApplyEndpoint(t *testing.T) {
	srv, cfg := testServer(t)
	body, _ := json.Marshal(openapi.ImportCSVRequest{
		Content: "section,target,kind,value,enabled,comment\n" +
			"domain,HydraRoute,domain,openai.com,true,\n" +
			"cidr,HydraRoute,cidr,8.8.8.8/32,true,\n",
	})
	rr := perform(srv, "POST", "/api/v1/import/csv/apply", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	domainData, err := os.ReadFile(cfg.DomainConf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(domainData, []byte("openai.com/HydraRoute\n")) {
		t.Fatalf("domain config not updated:\n%s", string(domainData))
	}
	cidrData, err := os.ReadFile(cfg.CIDRList)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cidrData, []byte("8.8.8.8/32\n")) {
		t.Fatalf("cidr config not updated:\n%s", string(cidrData))
	}
}
