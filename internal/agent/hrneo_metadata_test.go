package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestHRNeoMetadataCoversAllConfigKeys(t *testing.T) {
	params := hrneoParamMetadata()
	if len(params) != len(hrneoConfigOrder) {
		t.Fatalf("expected %d params, got %d", len(hrneoConfigOrder), len(params))
	}
	for i, key := range hrneoConfigOrder {
		if params[i].Name != key {
			t.Fatalf("param[%d]: expected %q, got %q", i, key, params[i].Name)
		}
	}
}

func TestHRNeoDefaultPreviewAndGenerate(t *testing.T) {
	srv, cfg := testServer(t)
	before := []byte("# custom\nFutureKey=value\n")
	if err := os.WriteFile(cfg.HRNeoConf, before, 0o600); err != nil {
		t.Fatal(err)
	}

	rr := perform(srv, "GET", "/api/v1/config/hrneo/default", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"requiredAction":"restart"`)) {
		t.Fatalf("unexpected preview body=%s", rr.Body.String())
	}
	afterPreview, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterPreview, before) {
		t.Fatalf("preview changed config: %q", afterPreview)
	}

	rr = performWithHeaders(srv, "POST", "/api/v1/config/hrneo/generate-default", "test-token", nil, map[string]string{"If-Match": "stale"})
	if rr.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale status=%d body=%s", rr.Code, rr.Body.String())
	}
	afterStale, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterStale, before) {
		t.Fatalf("stale write changed config: %q", afterStale)
	}

	rr = perform(srv, "POST", "/api/v1/config/hrneo/generate-default", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", rr.Code, rr.Body.String())
	}
	generated, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	want := renderHRNeoConfig(defaultHRNeoConfig())
	if string(generated) != want {
		t.Fatalf("unexpected generated config:\nwant=%q\n got=%q", want, generated)
	}
	if strings.Count(string(generated), "\n") != 27 {
		t.Fatalf("expected 27 generated keys:\n%s", generated)
	}
}

func TestHRNeoMetadataEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/metadata/hrneo-params", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp openapi.HRNeoParamMetadataResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.SupportedHrneoVersion != "3.11.0-1" || len(resp.Params) != 27 {
		t.Fatalf("unexpected metadata: %#v", resp)
	}
}
