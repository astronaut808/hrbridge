package agent

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

const supportedHRNeoVersion = "3.11.0-1"

func (s *Server) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.compatibilityReport())
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Server) compatibilityReport() openapi.CompatibilityReport {
	var checks []openapi.CompatibilityCheck
	add := func(severity openapi.CompatibilityCheckSeverity, component openapi.CompatibilityCheckComponent, code, message, path, target string) {
		check := openapi.CompatibilityCheck{
			Severity:  severity,
			Component: component,
			Code:      code,
			Message:   message,
		}
		if path != "" {
			check.Path = &path
		}
		if target != "" {
			check.Target = &target
		}
		checks = append(checks, check)
	}

	add(openapi.CompatibilityCheckSeverityOk, openapi.CompatibilityCheckComponentBridge,
		"supported-hrneo-version", "HydraBridge compatibility target is HR Neo "+supportedHRNeoVersion, "", "")

	hrneo, unknown, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil {
		add(openapi.CompatibilityCheckSeverityError, openapi.CompatibilityCheckComponentHrneo,
			"hrneo-config-unreadable", err.Error(), s.cfg.HRNeoConf, "")
	} else {
		add(openapi.CompatibilityCheckSeverityOk, openapi.CompatibilityCheckComponentHrneo,
			"hrneo-config-readable", "hrneo.conf is readable", s.cfg.HRNeoConf, "")
		if err := validateHRNeoConfig(hrneo); err != nil {
			add(openapi.CompatibilityCheckSeverityError, openapi.CompatibilityCheckComponentHrneo,
				"invalid-hrneo-config", err.Error(), s.cfg.HRNeoConf, "")
		}
		for _, key := range missingHRNeoKeys(hrneo) {
			add(openapi.CompatibilityCheckSeverityWarning, openapi.CompatibilityCheckComponentHrneo,
				"missing-hrneo-key", fmt.Sprintf("%s is missing; HR Neo will use its built-in default", key), s.cfg.HRNeoConf, "")
		}
		for _, key := range sortedMapKeys(unknown) {
			add(openapi.CompatibilityCheckSeverityWarning, openapi.CompatibilityCheckComponentHrneo,
				"unknown-hrneo-key", fmt.Sprintf("%s is not known to HydraBridge %s and will be preserved", key, Version), s.cfg.HRNeoConf, "")
		}
		checkGeoDataPaths(&checks, hrneo)
	}

	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil {
		add(openapi.CompatibilityCheckSeverityError, openapi.CompatibilityCheckComponentDomains,
			"domain-config-unreadable", err.Error(), s.cfg.DomainConf, "")
	} else {
		add(openapi.CompatibilityCheckSeverityOk, openapi.CompatibilityCheckComponentDomains,
			"domain-config-readable", "domain.conf is readable", s.cfg.DomainConf, "")
		appendValidationChecks(&checks, openapi.CompatibilityCheckComponentDomains, s.cfg.DomainConf, validateDomainConfigDeep(domains))
	}

	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil {
		add(openapi.CompatibilityCheckSeverityError, openapi.CompatibilityCheckComponentCidr,
			"cidr-config-unreadable", err.Error(), s.cfg.CIDRList, "")
	} else {
		add(openapi.CompatibilityCheckSeverityOk, openapi.CompatibilityCheckComponentCidr,
			"cidr-config-readable", "ip.list is readable", s.cfg.CIDRList, "")
		appendValidationChecks(&checks, openapi.CompatibilityCheckComponentCidr, s.cfg.CIDRList, validateCIDRConfigDeep(cidr))
	}

	ok := true
	for _, check := range checks {
		if check.Severity == openapi.CompatibilityCheckSeverityError {
			ok = false
			break
		}
	}
	return openapi.CompatibilityReport{
		Ok:                    ok,
		SupportedHrneoVersion: supportedHRNeoVersion,
		Checks:                checks,
	}
}

func appendValidationChecks(checks *[]openapi.CompatibilityCheck, component openapi.CompatibilityCheckComponent, filePath string, issues []openapi.ValidationIssue) {
	for _, issue := range issues {
		severity := openapi.CompatibilityCheckSeverityWarning
		if issue.Severity == openapi.Error {
			severity = openapi.CompatibilityCheckSeverityError
		}
		check := openapi.CompatibilityCheck{
			Severity:  severity,
			Component: component,
			Code:      issue.Code,
			Message:   issue.Message,
			Path:      &filePath,
			Target:    issue.Target,
		}
		*checks = append(*checks, check)
	}
}

func checkGeoDataPaths(checks *[]openapi.CompatibilityCheck, cfg openapi.HRNeoConfig) {
	add := func(severity openapi.CompatibilityCheckSeverity, code, message, path string) {
		check := openapi.CompatibilityCheck{
			Severity:  severity,
			Component: openapi.CompatibilityCheckComponentGeodata,
			Code:      code,
			Message:   message,
			Path:      &path,
		}
		*checks = append(*checks, check)
	}
	for _, path := range stringSliceValue(cfg.GeoIPFile) {
		checkGeoDataPath(path, "GeoIPFile", add)
	}
	for _, path := range stringSliceValue(cfg.GeoSiteFile) {
		checkGeoDataPath(path, "GeoSiteFile", add)
	}
}

func checkGeoDataPath(path, key string, add func(openapi.CompatibilityCheckSeverity, string, string, string)) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			add(openapi.CompatibilityCheckSeverityWarning, "geodata-file-missing", key+" file does not exist", path)
			return
		}
		add(openapi.CompatibilityCheckSeverityWarning, "geodata-file-unreadable", err.Error(), path)
		return
	}
	add(openapi.CompatibilityCheckSeverityOk, "geodata-file-exists", key+" file exists", path)
}

func missingHRNeoKeys(cfg openapi.HRNeoConfig) []string {
	var missing []string
	for _, key := range hrneoConfigOrder {
		if !hrneoConfigHasKey(cfg, key) {
			missing = append(missing, key)
		}
	}
	return missing
}

func hrneoConfigHasKey(cfg openapi.HRNeoConfig, key string) bool {
	switch key {
	case "autoStart":
		return cfg.AutoStart != nil
	case "watchlistPath":
		return cfg.WatchlistPath != nil
	case "clearIPSet":
		return cfg.ClearIPSet != nil
	case "CIDR":
		return cfg.CIDR != nil
	case "CIDRfile":
		return cfg.CIDRfile != nil
	case "IpsetEnableTimeout":
		return cfg.IpsetEnableTimeout != nil
	case "IpsetTimeout":
		return cfg.IpsetTimeout != nil
	case "log":
		return cfg.Log != nil
	case "logfile":
		return cfg.Logfile != nil
	case "DirectRouteEnabled":
		return cfg.DirectRouteEnabled != nil
	case "InterfaceFwMarkStart":
		return cfg.InterfaceFwMarkStart != nil
	case "InterfaceTableStart":
		return cfg.InterfaceTableStart != nil
	case "GlobalRouting":
		return cfg.GlobalRouting != nil
	case "ConntrackFlush":
		return cfg.ConntrackFlush != nil
	case "IpsetMaxElem":
		return cfg.IpsetMaxElem != nil
	case "GeoIPFile":
		return cfg.GeoIPFile != nil
	case "GeoSiteFile":
		return cfg.GeoSiteFile != nil
	case "PolicyOrder":
		return cfg.PolicyOrder != nil
	case "l7CaptureEnabled":
		return cfg.L7CaptureEnabled != nil
	case "l7QueueNum":
		return cfg.L7QueueNum != nil
	case "l7EnableTLS":
		return cfg.L7EnableTLS != nil
	case "l7EnableHTTP":
		return cfg.L7EnableHTTP != nil
	case "l7WanInterface":
		return cfg.L7WanInterface != nil
	case "l7ConnbytesMax":
		return cfg.L7ConnbytesMax != nil
	case "l7TcpReasmEnabled":
		return cfg.L7TcpReasmEnabled != nil
	case "l7TcpReasmMaxEntries":
		return cfg.L7TcpReasmMaxEntries != nil
	case "l7TcpReasmTtlSec":
		return cfg.L7TcpReasmTtlSec != nil
	default:
		return true
	}
}
