package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestDiagnoseDomainUsesPolicyOrderAndSpecificity(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("PolicyOrder=OtherPolicy,HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DomainConf, []byte("example.com/HydraRoute\nsub.example.com/OtherPolicy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(openapi.DomainDiagnosticRequest{Domain: "www.sub.example.com"})
	rr := perform(srv, "POST", "/api/v1/diagnostics/domain", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"matched":true`),
		[]byte(`"target":"OtherPolicy"`),
		[]byte(`"matchedDomain":"sub.example.com"`),
		[]byte(`"matchType":"suffix"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestDiagnoseIPUsesTargetOrder(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("PolicyOrder=OtherPolicy,HydraRoute\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/HydraRoute\n10.0.0.0/8\n\n/OtherPolicy\n10.1.0.0/16\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(openapi.IPDiagnosticRequest{Ip: "10.1.2.3"})
	rr := perform(srv, "POST", "/api/v1/diagnostics/ip", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"matched":true`),
		[]byte(`"target":"OtherPolicy"`),
		[]byte(`"prefix":"10.1.0.0/16"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestDiagnoseIPIncludesGeoIPAndRuntimeEvidence(t *testing.T) {
	_, cfg := testServer(t)
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "ipset"), `#!/bin/sh
if [ "$1" = "list" ]; then
	printf 'Finland\nFinlandv6\nRussia\nRussiav6\n'
	exit 0
fi
if [ "$1" = "test" ] && [ "$2" = "Finland" ] && [ "$3" = "1.1.1.42" ]; then
	exit 0
fi
exit 1
`)
	writeExecutable(t, filepath.Join(bin, "iptables"), `#!/bin/sh
printf '%s\n' '-A PREROUTING -m set --match-set Finland dst -j CONNMARK --set-xmark 0xffffaaa/0xffffffff'
`)
	writeExecutable(t, filepath.Join(bin, "ip6tables"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"Finland":{"mark":"ffffaaa"},"Russia":{"mark":"ffffaab"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	geoip := filepath.Join(t.TempDir(), "geoip.dat")
	if err := os.WriteFile(geoip, geoIPDataDAT("RU", "1.1.1.0/24"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.RCIURL = "http://rci.test"
	if err := os.WriteFile(cfg.HRNeoConf, []byte("PolicyOrder=Finland,Russia\nGeoIPFile="+geoip+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/Finland\ngeoip:RU\n\n/Russia\n1.1.1.42/32\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(cfg)

	body, _ := json.Marshal(openapi.IPDiagnosticRequest{Ip: "1.1.1.42"})
	rr := perform(srv, "POST", "/api/v1/diagnostics/ip", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"geoipMatches":[{"prefix":"1.1.1.0/24","prefixBits":24,"priority":0,"tag":"RU","target":"Finland"}]`),
		[]byte(`"ipsetAvailable":true`),
		[]byte(`"policyMarksAvailable":true`),
		[]byte(`"firewallAvailable":true`),
		[]byte(`"matched":true`),
		[]byte(`"setName":"Finland"`),
		[]byte(`"policyMark":"ffffaaa"`),
		[]byte(`--match-set Finland dst`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func TestDiagnoseIPPromotesRuntimeOnlyMatchToTopLevel(t *testing.T) {
	_, cfg := testServer(t)
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "ipset"), `#!/bin/sh
if [ "$1" = "list" ]; then
	printf 'Finland\nFinlandv6\n'
	exit 0
fi
if [ "$1" = "test" ] && [ "$2" = "Finland" ] && [ "$3" = "172.64.155.209" ]; then
	exit 0
fi
exit 1
`)
	writeExecutable(t, filepath.Join(bin, "iptables"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(bin, "ip6tables"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"Finland":{"mark":"ffffaaa"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	cfg.RCIURL = "http://rci.test"
	if err := os.WriteFile(cfg.HRNeoConf, []byte("PolicyOrder=Finland\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CIDRList, []byte("/Finland\n1.1.1.1/32\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(cfg)

	body, _ := json.Marshal(openapi.IPDiagnosticRequest{Ip: "172.64.155.209"})
	rr := perform(srv, "POST", "/api/v1/diagnostics/ip", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for _, want := range [][]byte{
		[]byte(`"candidates":[]`),
		[]byte(`"geoipMatches":[]`),
		[]byte(`"matched":true`),
		[]byte(`"target":"Finland"`),
		[]byte(`"setName":"Finland"`),
	} {
		if !bytes.Contains(rr.Body.Bytes(), want) {
			t.Fatalf("missing %s in %s", want, rr.Body.String())
		}
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}
