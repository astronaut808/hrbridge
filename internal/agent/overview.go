package agent

import (
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	status := openapi.StatusResponse{Hrneo: s.hrneoStatus()}
	status.Hrbridge.Version = Version
	status.Hrbridge.UptimeSec = int64(sinceSeconds(s.started))
	status.Paths.HrneoConf = s.cfg.HRNeoConf
	status.Paths.DomainConf = s.cfg.DomainConf
	status.Paths.CidrList = s.cfg.CIDRList
	status.Paths.BackupDir = s.cfg.BackupDir
	status.Paths.AuditLog = s.cfg.AuditLog

	inv, err := s.targetInventory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	compat := s.compatibilityReport()
	resp := openapi.OverviewResponse{
		Ok:              compat.Ok,
		Status:          status,
		TargetCount:     len(inv.Targets),
		DomainRuleCount: countDomainRules(domains),
		CidrRuleCount:   countCIDRRules(cidr),
	}
	for _, check := range compat.Checks {
		switch check.Severity {
		case openapi.CompatibilityCheckSeverityError:
			resp.ErrorCount++
		case openapi.CompatibilityCheckSeverityWarning:
			resp.WarningCount++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func countDomainRules(cfg openapi.DomainConfig) int {
	total := 0
	for _, target := range cfg.Targets {
		if target.Enabled {
			total += len(target.Domains) + len(target.Geosite)
		}
	}
	return total
}

func countCIDRRules(cfg openapi.CIDRConfig) int {
	total := 0
	for _, block := range cfg.Blocks {
		if block.Enabled {
			total += len(block.Entries) + len(block.Geoip)
		}
	}
	return total
}
