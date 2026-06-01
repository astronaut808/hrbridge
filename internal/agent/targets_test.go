package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTargetsEndpointReturnsInventory(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/targets", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"name":"HydraRoute"`)) {
		t.Fatalf("missing HydraRoute target: %s", rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"domainRules":1`)) {
		t.Fatalf("missing domain count: %s", rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"cidrRules":1`)) {
		t.Fatalf("missing cidr count: %s", rr.Body.String())
	}
}

func TestTargetInterfacesEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	rr := perform(srv, "GET", "/api/v1/targets/interfaces", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"interfaces"`)) {
		t.Fatalf("unexpected body=%s", rr.Body.String())
	}
}

func TestTargetPoliciesEndpoint(t *testing.T) {
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/rci/show/ip/policy/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"HydraRoute":{"mark":"0x21"},"Other":{"mark":"0x22"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, cfg := testServer(t)
	cfg.RCIURL = "http://rci.test"
	srv := NewServer(cfg)

	rr := perform(srv, "GET", "/api/v1/targets/policies", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"name":"HydraRoute"`)) {
		t.Fatalf("missing policy: %s", rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"mark":"0x21"`)) {
		t.Fatalf("missing mark: %s", rr.Body.String())
	}
}

func TestCreateTargetPolicyEndpoint(t *testing.T) {
	var bodies []string
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(data))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, cfg := testServer(t)
	cfg.RCIURL = "http://rci.test"
	srv := NewServer(cfg)

	body, _ := json.Marshal(openapi.PolicyMutationRequest{Name: "NewPolicy"})
	rr := perform(srv, "POST", "/api/v1/targets/policies", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(bodies) != 2 {
		t.Fatalf("expected command and save requests, got %d: %#v", len(bodies), bodies)
	}
	if !strings.Contains(bodies[0], `"parse":"ip policy NewPolicy"`) {
		t.Fatalf("unexpected command body: %s", bodies[0])
	}
	if !strings.Contains(bodies[1], `"save":true`) {
		t.Fatalf("unexpected save body: %s", bodies[1])
	}
}

func TestDeleteTargetPolicyEndpoint(t *testing.T) {
	var commandBody string
	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		if commandBody == "" {
			commandBody = string(data)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, cfg := testServer(t)
	cfg.RCIURL = "http://rci.test"
	srv := NewServer(cfg)

	rr := perform(srv, "DELETE", "/api/v1/targets/policies/OldPolicy", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(commandBody, `"parse":"no ip policy OldPolicy"`) {
		t.Fatalf("unexpected command body: %s", commandBody)
	}
}

func TestTargetOrderEndpointUpdatesPolicyOrder(t *testing.T) {
	srv, cfg := testServer(t)
	if err := os.WriteFile(cfg.HRNeoConf, []byte("# preserve me\nPolicyOrder=Old\nFutureKey=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(openapi.PutTargetOrderRequest{
		Order: []string{"HydraRoute", "nwg0"},
	})
	rr := perform(srv, "PUT", "/api/v1/targets/order", "test-token", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	data, err := os.ReadFile(cfg.HRNeoConf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "PolicyOrder=HydraRoute,nwg0\n") {
		t.Fatalf("missing PolicyOrder in:\n%s", string(data))
	}
	if string(data) != "# preserve me\nPolicyOrder=HydraRoute,nwg0\nFutureKey=value\n" {
		t.Fatalf("unrelated config text changed:\n%s", string(data))
	}

	rr = perform(srv, "GET", "/api/v1/targets/order", "test-token", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"HydraRoute"`)) {
		t.Fatalf("unexpected order body=%s", rr.Body.String())
	}
}

func TestTargetOrderRejectsDuplicates(t *testing.T) {
	srv, _ := testServer(t)
	body, _ := json.Marshal(openapi.PutTargetOrderRequest{
		Order: []string{"HydraRoute", "hydraroute"},
	})
	rr := perform(srv, "PUT", "/api/v1/targets/order", "test-token", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
