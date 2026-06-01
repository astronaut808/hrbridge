package agent

import (
	"net/http"
	"testing"
)

func TestRuntimeEndpointsReturnReadOnlyResponses(t *testing.T) {
	srv, _ := testServer(t)
	for _, path := range []string{
		"/api/v1/runtime/ipsets",
		"/api/v1/runtime/firewall",
		"/api/v1/runtime/direct-routes",
	} {
		rr := perform(srv, "GET", path, "test-token", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rr.Code, rr.Body.String())
		}
	}
}
