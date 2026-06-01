package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type cidrEntryRef struct {
	target string
	path   string
	prefix netip.Prefix
}

func (s *Server) handleValidateDomains(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutDomainConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, validationResponse(validateDomainConfigDeep(req.Config)))
}

func (s *Server) handleValidateCIDR(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutCIDRConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, validationResponse(validateCIDRConfigDeep(req.Config)))
}

func validateHRNeoConfig(cfg openapi.HRNeoConfig) error {
	if cfg.Log != nil && !cfg.Log.Valid() {
		return fmt.Errorf("config.log must be one of: console, file, syslog, off")
	}
	for name, value := range map[string]*int{
		"IpsetTimeout": cfg.IpsetTimeout,
	} {
		if value != nil && *value < 0 {
			return fmt.Errorf("config.%s must be greater than or equal to 0", name)
		}
	}
	for name, value := range map[string]*int{
		"IpsetMaxElem":         cfg.IpsetMaxElem,
		"InterfaceFwMarkStart": cfg.InterfaceFwMarkStart,
		"InterfaceTableStart":  cfg.InterfaceTableStart,
		"l7QueueNum":           cfg.L7QueueNum,
		"l7ConnbytesMax":       cfg.L7ConnbytesMax,
		"l7TcpReasmMaxEntries": cfg.L7TcpReasmMaxEntries,
		"l7TcpReasmTtlSec":     cfg.L7TcpReasmTtlSec,
	} {
		if value != nil && *value <= 0 {
			return fmt.Errorf("config.%s must be greater than 0", name)
		}
	}
	for name, value := range map[string]*string{
		"watchlistPath":  cfg.WatchlistPath,
		"CIDRfile":       cfg.CIDRfile,
		"logfile":        cfg.Logfile,
		"l7WanInterface": cfg.L7WanInterface,
	} {
		if value != nil && hasLineBreak(*value) {
			return fmt.Errorf("config.%s must not contain line breaks", name)
		}
	}
	if err := validateStringList("GeoIPFile", cfg.GeoIPFile, validatePathValue); err != nil {
		return err
	}
	if err := validateStringList("GeoSiteFile", cfg.GeoSiteFile, validatePathValue); err != nil {
		return err
	}
	return validateStringList("PolicyOrder", cfg.PolicyOrder, validateTargetName)
}

func validateDomainConfig(cfg openapi.DomainConfig) error {
	for i, target := range cfg.Targets {
		if err := validateTargetName(target.Name); err != nil {
			return fmt.Errorf("targets[%d].name: %w", i, err)
		}
		if target.Type != nil && !target.Type.Valid() {
			return fmt.Errorf("targets[%d].type must be one of: unknown, policy, interface", i)
		}
		if target.Enabled && len(target.Domains) == 0 && len(target.Geosite) == 0 {
			return fmt.Errorf("targets[%d] must include at least one domain or geosite tag", i)
		}
		for j, domain := range target.Domains {
			if err := validateDomainValue(domain); err != nil {
				return fmt.Errorf("targets[%d].domains[%d]: %w", i, j, err)
			}
		}
		for j, tag := range target.Geosite {
			if err := validateTagValue(tag); err != nil {
				return fmt.Errorf("targets[%d].geosite[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

func validateCIDRConfig(cfg openapi.CIDRConfig) error {
	for i, block := range cfg.Blocks {
		if err := validateTargetName(block.Target); err != nil {
			return fmt.Errorf("blocks[%d].target: %w", i, err)
		}
		if block.Enabled && len(block.Entries) == 0 && len(block.Geoip) == 0 {
			return fmt.Errorf("blocks[%d] must include at least one CIDR/IP entry or geoip tag", i)
		}
		for j, tag := range block.Geoip {
			if err := validateTagValue(tag); err != nil {
				return fmt.Errorf("blocks[%d].geoip[%d]: %w", i, j, err)
			}
		}
		for j, entry := range block.Entries {
			if err := validateIPOrPrefix(entry); err != nil {
				return fmt.Errorf("blocks[%d].entries[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

func validateDomainConfigDeep(cfg openapi.DomainConfig) []openapi.ValidationIssue {
	var issues []openapi.ValidationIssue
	if err := validateDomainConfig(cfg); err != nil {
		issues = append(issues, validationIssue(openapi.Error, "invalid-domain-config", err.Error(), "", ""))
		return issues
	}

	seenDomains := map[string]string{}
	seenGeo := map[string]string{}
	emitted := map[string]bool{}
	for i, target := range cfg.Targets {
		if !target.Enabled {
			continue
		}
		for j, domain := range target.Domains {
			key := strings.ToLower(strings.TrimSpace(domain))
			path := fmt.Sprintf("targets[%d].domains[%d]", i, j)
			if prev, ok := seenDomains[key]; ok {
				code := "duplicate-domain"
				if prev != target.Name {
					code = "conflicting-domain"
				}
				issueKey := strings.Join([]string{code, key, prev, target.Name}, "\x00")
				if emitted[issueKey] {
					continue
				}
				emitted[issueKey] = true
				issues = append(issues, validationIssue(openapi.Warning, code,
					fmt.Sprintf("domain %q is also routed to %q", key, prev), target.Name, path))
				continue
			}
			seenDomains[key] = target.Name
		}
		for j, tag := range target.Geosite {
			key := strings.ToLower(strings.TrimSpace(tag))
			path := fmt.Sprintf("targets[%d].geosite[%d]", i, j)
			if prev, ok := seenGeo[key]; ok {
				code := "duplicate-geosite"
				if prev != target.Name {
					code = "conflicting-geosite"
				}
				issueKey := strings.Join([]string{code, key, prev, target.Name}, "\x00")
				if emitted[issueKey] {
					continue
				}
				emitted[issueKey] = true
				issues = append(issues, validationIssue(openapi.Warning, code,
					fmt.Sprintf("geosite:%s is also routed to %q", key, prev), target.Name, path))
				continue
			}
			seenGeo[key] = target.Name
		}
	}
	return issues
}

func validateCIDRConfigDeep(cfg openapi.CIDRConfig) []openapi.ValidationIssue {
	var issues []openapi.ValidationIssue
	if err := validateCIDRConfig(cfg); err != nil {
		issues = append(issues, validationIssue(openapi.Error, "invalid-cidr-config", err.Error(), "", ""))
		return issues
	}

	var entries []cidrEntryRef
	seenGeo := map[string]string{}
	for i, block := range cfg.Blocks {
		if !block.Enabled {
			continue
		}
		for j, tag := range block.Geoip {
			key := strings.ToLower(strings.TrimSpace(tag))
			path := fmt.Sprintf("blocks[%d].geoip[%d]", i, j)
			if prev, ok := seenGeo[key]; ok {
				code := "duplicate-geoip"
				if prev != block.Target {
					code = "conflicting-geoip"
				}
				issues = append(issues, validationIssue(openapi.Warning, code,
					fmt.Sprintf("geoip:%s is also routed to %q", key, prev), block.Target, path))
				continue
			}
			seenGeo[key] = block.Target
		}
		for j, entry := range block.Entries {
			prefix := normalizePrefix(strings.TrimSpace(entry))
			path := fmt.Sprintf("blocks[%d].entries[%d]", i, j)
			for _, prev := range entries {
				if !prefixesOverlap(prefix, prev.prefix) {
					continue
				}
				code := "overlapping-cidr"
				if prefix == prev.prefix {
					code = "duplicate-cidr"
				}
				if prev.target != block.Target {
					code = "conflicting-cidr"
				}
				issues = append(issues, validationIssue(openapi.Warning, code,
					fmt.Sprintf("%s overlaps with %s routed to %q", prefix.String(), prev.prefix.String(), prev.target),
					block.Target, path))
			}
			entries = append(entries, cidrEntryRef{target: block.Target, path: path, prefix: prefix})
		}
	}
	return issues
}

func validationResponse(issues []openapi.ValidationIssue) openapi.ValidationResponse {
	if issues == nil {
		issues = []openapi.ValidationIssue{}
	}
	ok := true
	for _, issue := range issues {
		if issue.Severity == openapi.Error {
			ok = false
			break
		}
	}
	return openapi.ValidationResponse{Ok: ok, Issues: issues}
}

func validationIssue(severity openapi.ValidationIssueSeverity, code, message, target, path string) openapi.ValidationIssue {
	issue := openapi.ValidationIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
	}
	if target != "" {
		issue.Target = &target
	}
	if path != "" {
		issue.Path = &path
	}
	return issue
}

func normalizePrefix(value string) netip.Prefix {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked()
	}
	addr, _ := netip.ParseAddr(value)
	bits := 128
	if addr.Is4() {
		bits = 32
	}
	return netip.PrefixFrom(addr, bits)
}

func prefixesOverlap(a, b netip.Prefix) bool {
	if a.Addr().Is4() != b.Addr().Is4() {
		return false
	}
	return a.Contains(b.Addr()) || b.Contains(a.Addr())
}

func validateStringList(name string, values *[]string, validate func(string) error) error {
	if values == nil {
		return nil
	}
	for i, value := range *values {
		if err := validate(value); err != nil {
			return fmt.Errorf("config.%s[%d]: %w", name, i, err)
		}
	}
	return nil
}

func validatePathValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if hasLineBreak(value) {
		return fmt.Errorf("must not contain line breaks")
	}
	return nil
}

func validateTargetName(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.ContainsAny(value, "\r\n/,") {
		return fmt.Errorf("must not contain '/', ',' or line breaks")
	}
	return nil
}

func validateDomainValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.ContainsAny(value, "\r\n/, ") {
		return fmt.Errorf("must not contain spaces, '/', ',' or line breaks")
	}
	return nil
}

func validateTagValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.ContainsAny(value, "\r\n/, ") {
		return fmt.Errorf("must not contain spaces, '/', ',' or line breaks")
	}
	return nil
}

func validateIPOrPrefix(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if hasLineBreak(value) {
		return fmt.Errorf("must not contain line breaks")
	}
	if _, err := netip.ParsePrefix(value); err == nil {
		return nil
	}
	return fmt.Errorf("must be a CIDR prefix with an explicit mask, for example 1.1.1.1/32 or 2001:db8::1/128")
}

func hasLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}
