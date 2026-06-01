package agent

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	cfg     Config
	started time.Time
	mux     *http.ServeMux
}

const maxRequestBodyBytes = 128 << 20

func NewServer(cfg Config) *http.Server {
	s := &Server{
		cfg:     cfg,
		started: time.Now(),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return &http.Server{
		Addr:              cfg.Listen,
		Handler:           s.withCommonMiddleware(s.mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func (s *Server) routes() {
	s.handle("GET /api/v1/health", s.public(s.handleHealth))
	s.handle("GET /api/v1/version", s.auth(s.handleVersion))
	s.handle("GET /api/v1/status", s.auth(s.handleStatus))
	s.handle("GET /api/v1/overview", s.auth(s.handleOverview))
	s.handle("POST /api/v1/auth/token/rotate", s.auth(s.handleRotateToken))
	s.handle("GET /api/v1/compatibility", s.auth(s.handleCompatibility))
	s.handle("GET /api/v1/doctor", s.auth(s.handleDoctor))
	s.handle("GET /api/v1/config/hrneo", s.auth(s.handleGetConfig("hrneo")))
	s.handle("PUT /api/v1/config/hrneo", s.auth(s.handlePutConfig("hrneo")))
	s.handle("GET /api/v1/config/hrneo/structured", s.auth(s.handleGetHRNeoStructured))
	s.handle("PUT /api/v1/config/hrneo/structured", s.auth(s.handlePutHRNeoStructured))
	s.handle("GET /api/v1/config/hrneo/default", s.auth(s.handleGetHRNeoDefault))
	s.handle("POST /api/v1/config/hrneo/generate-default", s.auth(s.handleGenerateHRNeoDefault))
	s.handle("GET /api/v1/metadata/hrneo-params", s.auth(s.handleHRNeoParamMetadata))
	s.handle("GET /api/v1/config/domains", s.auth(s.handleGetConfig("domains")))
	s.handle("PUT /api/v1/config/domains", s.auth(s.handlePutConfig("domains")))
	s.handle("GET /api/v1/config/domains/structured", s.auth(s.handleGetDomainsStructured))
	s.handle("PUT /api/v1/config/domains/structured", s.auth(s.handlePutDomainsStructured))
	s.handle("POST /api/v1/config/domains/validate", s.auth(s.handleValidateDomains))
	s.handle("POST /api/v1/config/domains/targets/{target}/rules", s.auth(s.handlePatchDomainRule(true)))
	s.handle("DELETE /api/v1/config/domains/targets/{target}/rules", s.auth(s.handlePatchDomainRule(false)))
	s.handle("GET /api/v1/config/cidr", s.auth(s.handleGetConfig("cidr")))
	s.handle("PUT /api/v1/config/cidr", s.auth(s.handlePutConfig("cidr")))
	s.handle("GET /api/v1/config/cidr/structured", s.auth(s.handleGetCIDRStructured))
	s.handle("PUT /api/v1/config/cidr/structured", s.auth(s.handlePutCIDRStructured))
	s.handle("POST /api/v1/config/cidr/validate", s.auth(s.handleValidateCIDR))
	s.handle("POST /api/v1/config/cidr/targets/{target}/rules", s.auth(s.handlePatchCIDRRule(true)))
	s.handle("DELETE /api/v1/config/cidr/targets/{target}/rules", s.auth(s.handlePatchCIDRRule(false)))
	s.handle("GET /api/v1/views/domains/grouped", s.auth(s.handleGroupedDomains))
	s.handle("GET /api/v1/views/cidr/grouped", s.auth(s.handleGroupedCIDR))
	s.handle("GET /api/v1/targets", s.auth(s.handleTargets))
	s.handle("GET /api/v1/targets/interfaces", s.auth(s.handleTargetInterfaces))
	s.handle("GET /api/v1/targets/policies", s.auth(s.handleTargetPolicies))
	s.handle("POST /api/v1/targets/policies", s.auth(s.handleCreateTargetPolicy))
	s.handle("DELETE /api/v1/targets/policies/{name}", s.auth(s.handleDeleteTargetPolicy))
	s.handle("GET /api/v1/targets/order", s.auth(s.handleGetTargetOrder))
	s.handle("PUT /api/v1/targets/order", s.auth(s.handlePutTargetOrder))
	s.handle("GET /api/v1/geodata/files", s.auth(s.handleGeoDataFiles))
	s.handle("POST /api/v1/geodata/files", s.auth(s.handleGeoDataUpload))
	s.handle("POST /api/v1/geodata/download", s.auth(s.handleGeoDataDownload))
	s.handle("GET /api/v1/geodata/references", s.auth(s.handleGeoDataReferences))
	s.handle("GET /api/v1/geodata/tags", s.auth(s.handleGeoDataTags))
	s.handle("GET /api/v1/geodata/validate", s.auth(s.handleGeoDataValidate))
	s.handle("POST /api/v1/geodata/validate", s.auth(s.handleGeoDataValidate))
	s.handle("GET /api/v1/export/csv", s.auth(s.handleExportCSV))
	s.handle("POST /api/v1/import/csv/preview", s.auth(s.handleImportCSVPreview))
	s.handle("POST /api/v1/import/csv/apply", s.auth(s.handleImportCSVApply))
	s.handle("POST /api/v1/imports/text/preview", s.auth(s.handleImportTextPreview))
	s.handle("POST /api/v1/imports/text/apply", s.auth(s.handleImportTextApply))
	s.handle("POST /api/v1/diagnostics/domain", s.auth(s.handleDiagnoseDomain))
	s.handle("POST /api/v1/diagnostics/ip", s.auth(s.handleDiagnoseIP))
	s.handle("GET /api/v1/runtime/ipsets", s.auth(s.handleRuntimeIPSets))
	s.handle("GET /api/v1/runtime/firewall", s.auth(s.handleRuntimeFirewall))
	s.handle("GET /api/v1/runtime/policies", s.auth(s.handleTargetPolicies))
	s.handle("GET /api/v1/runtime/direct-routes", s.auth(s.handleRuntimeDirectRoutes))
	s.handle("POST /api/v1/service/start", s.auth(s.handleService("start")))
	s.handle("POST /api/v1/service/stop", s.auth(s.handleService("stop")))
	s.handle("POST /api/v1/service/restart", s.auth(s.handleService("restart")))
	s.handle("POST /api/v1/service/reload", s.auth(s.handleService("reload")))
	s.handle("GET /api/v1/logs", s.auth(s.handleLogs))
	s.handle("GET /api/v1/audit", s.auth(s.handleAudit))
	s.handle("GET /api/v1/backups", s.auth(s.handleListBackups))
	s.handle("POST /api/v1/backups", s.auth(s.handleCreateBackup))
	s.handle("POST /api/v1/backups/restore", s.auth(s.handleRestoreBackup))
}

func (s *Server) handle(pattern string, h http.HandlerFunc) {
	s.mux.HandleFunc(pattern, h)
}

func (s *Server) public(h http.HandlerFunc) http.HandlerFunc {
	return h
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken == "" {
			writeError(w, http.StatusServiceUnavailable, "auth token is not configured")
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		got := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AuthToken)) != 1 {
			writeError(w, http.StatusForbidden, "invalid token")
			return
		}
		h(w, r)
	}
}

func (s *Server) withCommonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		if s.cfg.AllowOrigins != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowOrigins)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
