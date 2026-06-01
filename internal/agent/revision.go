package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
)

func fileRevision(path string) string {
	data, err := os.ReadFile(path) // #nosec G304 -- callers select a configured HR Neo file from a fixed allowlist
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func setRevisionHeader(w http.ResponseWriter, revision string) {
	if revision == "" {
		return
	}
	w.Header().Set("ETag", `"`+revision+`"`)
	w.Header().Set("X-Config-Revision", revision)
}

func checkRevisionPrecondition(r *http.Request, path string) bool {
	want := strings.Trim(r.Header.Get("If-Match"), `"`)
	if want == "" {
		return true
	}
	return want == fileRevision(path)
}
