package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestImportTextPreviewDomain(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.ImportTextRequest{
		Section: openapi.ImportTextRequestSectionDomain,
		Target:  "HydraRoute",
		Content: "example.com\ngeosite:youtube\n",
	})
	rr := perform(srv, "POST", "/api/v1/imports/text/preview", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"domains":["example.com"]`),
		[]byte(`"geosite":["youtube"]`),
		[]byte(`"ok":true`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestImportTextApplyCIDR(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.ImportTextRequest{
		Section: openapi.ImportTextRequestSectionCidr,
		Target:  "HydraRoute",
		Content: "8.8.8.8/32\ngeoip:US\n",
	})
	rr := perform(srv, "POST", "/api/v1/imports/text/apply", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"saved":true`),
		[]byte(`"entries":["8.8.8.8/32"]`),
		[]byte(`"geoip":["US"]`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}
