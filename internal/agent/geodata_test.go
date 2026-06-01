package agent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestGeoDataUploadUpdatesConfig(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("# preserve me\nFutureKey=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(cfg.HRNeoConf), "geofile", "geoip.dat")
	body, _ := json.Marshal(openapi.GeoDataUploadRequest{
		Kind:          openapi.GeoDataKindGeoip,
		Path:          path,
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("geoip-data")),
	})

	rr := perform(srv, "POST", "/api/v1/geodata/files", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "geoip-data" {
		t.Fatalf("unexpected data: %q", string(data))
	}
	conf, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(conf, []byte("GeoIPFile="+path+"\n")) {
		t.Fatalf("config missing GeoIPFile:\n%s", string(conf))
	}
	want := "# preserve me\nFutureKey=value\nGeoIPFile=" + path + "\n"
	if string(conf) != want {
		t.Fatalf("unrelated config text changed:\nwant=%q\n got=%q", want, string(conf))
	}
}

func TestGeoDataFilesEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/geodata/files", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"files"`)) {
		t.Fatalf("unexpected body=%s", rr.Body.String())
	}
}

func TestGeoDataDownload(t *testing.T) {
	oldClient := geoDataHTTPClient
	defer func() { geoDataHTTPClient = oldClient }()
	geoDataHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("geosite-data"))),
			Header:     make(http.Header),
		}, nil
	})}

	srv, cfg := testServer(t)
	path := filepath.Join(filepath.Dir(cfg.HRNeoConf), "geofile", "geosite.dat")
	body, _ := json.Marshal(openapi.GeoDataDownloadRequest{
		Kind: openapi.GeoDataKindGeosite,
		Path: path,
		Url:  "https://example.test/geosite.dat",
	})

	rr := perform(srv, "POST", "/api/v1/geodata/download", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "geosite-data" {
		t.Fatalf("unexpected data: %q", string(data))
	}
}

func TestGeoDataUploadRejectsPathOutsideGeoFileDirectory(t *testing.T) {
	srv, cfg := testServer(t)
	path := filepath.Join(filepath.Dir(cfg.HRNeoConf), "outside.dat")
	body, _ := json.Marshal(openapi.GeoDataUploadRequest{
		Kind:          openapi.GeoDataKindGeoip,
		Path:          path,
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("geoip-data")),
	})

	rr := perform(srv, "POST", "/api/v1/geodata/files", "test-token", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected file state: %v", err)
	}
}

func TestGeoDataDownloadRejectsPrivateAddress(t *testing.T) {
	srv, cfg := testServer(t)
	path := filepath.Join(filepath.Dir(cfg.HRNeoConf), "geofile", "geoip.dat")
	body, _ := json.Marshal(openapi.GeoDataDownloadRequest{
		Kind: openapi.GeoDataKindGeoip,
		Path: path,
		Url:  "http://127.0.0.1/geodata.dat",
	})

	rr := perform(srv, "POST", "/api/v1/geodata/download", "test-token", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected file state: %v", err)
	}
}
