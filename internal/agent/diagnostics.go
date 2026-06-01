package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleDiagnoseDomain(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.DomainDiagnosticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.diagnoseDomain(req.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDiagnoseIP(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.IPDiagnosticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.diagnoseIP(r.Context(), req.Ip)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) diagnoseDomain(rawDomain string) (openapi.DomainDiagnosticResponse, error) {
	domain := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(rawDomain, ".")))
	if err := validateDomainValue(domain); err != nil {
		return openapi.DomainDiagnosticResponse{}, fmt.Errorf("domain: %w", err)
	}

	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil {
		return openapi.DomainDiagnosticResponse{}, err
	}
	hrneo, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.DomainDiagnosticResponse{}, err
	}
	order := diagnosticTargetOrder(hrneo, domains, openapi.CIDRConfig{})
	priority := priorityMap(order)

	resp := openapi.DomainDiagnosticResponse{
		Domain:      domain,
		Matched:     false,
		Candidates:  []openapi.DomainDiagnosticCandidate{},
		TargetOrder: order,
		Notes: []string{
			"GeoSite tags are not expanded by this config-level diagnostic.",
			"Runtime CNAME chains can make HR Neo match a related canonical name from DNS responses.",
		},
	}

	for _, target := range domains.Targets {
		if !target.Enabled {
			continue
		}
		for _, rule := range target.Domains {
			rule = strings.ToLower(strings.TrimSpace(rule))
			if rule == "" {
				continue
			}
			matchType, specificity, ok := domainRuleMatch(domain, rule)
			if !ok {
				continue
			}
			resp.Candidates = append(resp.Candidates, openapi.DomainDiagnosticCandidate{
				Target:      target.Name,
				Rule:        rule,
				MatchType:   matchType,
				Priority:    targetPriority(priority, target.Name),
				Specificity: specificity,
			})
		}
	}

	sort.SliceStable(resp.Candidates, func(i, j int) bool {
		a, b := resp.Candidates[i], resp.Candidates[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Specificity != b.Specificity {
			return a.Specificity > b.Specificity
		}
		return a.Rule < b.Rule
	})
	if len(resp.Candidates) > 0 {
		winner := resp.Candidates[0]
		resp.Matched = true
		resp.Target = &winner.Target
		resp.MatchedDomain = &winner.Rule
		mt := openapi.DomainDiagnosticResponseMatchType(winner.MatchType)
		resp.MatchType = &mt
	}
	return resp, nil
}

func (s *Server) diagnoseIP(ctx context.Context, rawIP string) (openapi.IPDiagnosticResponse, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(rawIP))
	if err != nil {
		return openapi.IPDiagnosticResponse{}, fmt.Errorf("ip: %w", err)
	}
	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil {
		return openapi.IPDiagnosticResponse{}, err
	}
	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.IPDiagnosticResponse{}, err
	}
	hrneo, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.IPDiagnosticResponse{}, err
	}
	order := diagnosticTargetOrder(hrneo, domains, cidr)
	priority := priorityMap(order)

	resp := openapi.IPDiagnosticResponse{
		Ip:           addr.String(),
		Matched:      false,
		Candidates:   []openapi.IPDiagnosticCandidate{},
		GeoipMatches: []openapi.IPDiagnosticGeoIPMatch{},
		Runtime:      s.ipRuntimeEvidence(ctx, addr, order),
		TargetOrder:  order,
		Notes: []string{
			"Runtime ipset membership proves current presence but does not preserve whether HR Neo learned an address from DNS, L7, CIDR, or GeoIP.",
			"When multiple target ipsets contain the same IP, the first target in HR Neo's mangle rule order wins.",
		},
	}
	appendCIDRDiagnosticMatches(&resp, cidr, addr, priority)
	appendGeoIPDiagnosticMatches(&resp, cidr, hrneo, addr, priority)
	sortIPDiagnosticMatches(&resp)
	applyConfiguredIPWinner(&resp)
	if resp.Runtime.Matched {
		resp.Matched = true
		resp.Target = resp.Runtime.Target
	}
	return resp, nil
}

func appendCIDRDiagnosticMatches(resp *openapi.IPDiagnosticResponse, cidr openapi.CIDRConfig, addr netip.Addr, priority map[string]int) {
	for _, block := range cidr.Blocks {
		if !block.Enabled {
			continue
		}
		for _, entry := range block.Entries {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
			if err != nil {
				continue
			}
			prefix = prefix.Masked()
			if !prefix.Contains(addr) {
				continue
			}
			resp.Candidates = append(resp.Candidates, openapi.IPDiagnosticCandidate{
				Target:     block.Target,
				Prefix:     prefix.String(),
				Priority:   targetPriority(priority, block.Target),
				PrefixBits: prefix.Bits(),
			})
		}
	}
}

func appendGeoIPDiagnosticMatches(resp *openapi.IPDiagnosticResponse, cidr openapi.CIDRConfig, hrneo openapi.HRNeoConfig, addr netip.Addr, priority map[string]int) {
	for _, block := range cidr.Blocks {
		if !block.Enabled {
			continue
		}
		for _, tag := range block.Geoip {
			prefixes, err := geoIPPrefixesContaining(stringSliceValue(hrneo.GeoIPFile), tag, addr)
			if err != nil {
				resp.Notes = append(resp.Notes, fmt.Sprintf("GeoIP evidence for geoip:%s is unavailable: %v", tag, err))
				continue
			}
			for _, prefix := range prefixes {
				resp.GeoipMatches = append(resp.GeoipMatches, openapi.IPDiagnosticGeoIPMatch{
					Target:     block.Target,
					Tag:        tag,
					Prefix:     prefix.String(),
					Priority:   targetPriority(priority, block.Target),
					PrefixBits: prefix.Bits(),
				})
			}
		}
	}
}

func sortIPDiagnosticMatches(resp *openapi.IPDiagnosticResponse) {
	sort.SliceStable(resp.Candidates, func(i, j int) bool {
		a, b := resp.Candidates[i], resp.Candidates[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.PrefixBits != b.PrefixBits {
			return a.PrefixBits > b.PrefixBits
		}
		return a.Prefix < b.Prefix
	})
	sort.SliceStable(resp.GeoipMatches, func(i, j int) bool {
		a, b := resp.GeoipMatches[i], resp.GeoipMatches[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.PrefixBits != b.PrefixBits {
			return a.PrefixBits > b.PrefixBits
		}
		if a.Tag != b.Tag {
			return a.Tag < b.Tag
		}
		return a.Prefix < b.Prefix
	})
}

type configIPMatch struct {
	target     string
	prefix     string
	priority   int
	prefixBits int
}

func applyConfiguredIPWinner(resp *openapi.IPDiagnosticResponse) {
	var matches []configIPMatch
	for _, candidate := range resp.Candidates {
		matches = append(matches, configIPMatch{
			target:     candidate.Target,
			prefix:     candidate.Prefix,
			priority:   candidate.Priority,
			prefixBits: candidate.PrefixBits,
		})
	}
	for _, match := range resp.GeoipMatches {
		matches = append(matches, configIPMatch{
			target:     match.Target,
			prefix:     match.Prefix,
			priority:   match.Priority,
			prefixBits: match.PrefixBits,
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].priority != matches[j].priority {
			return matches[i].priority < matches[j].priority
		}
		if matches[i].prefixBits != matches[j].prefixBits {
			return matches[i].prefixBits > matches[j].prefixBits
		}
		return matches[i].prefix < matches[j].prefix
	})
	if len(matches) > 0 {
		winner := matches[0]
		resp.Matched = true
		resp.Target = &winner.target
		resp.Prefix = &winner.prefix
	}
}

func (s *Server) ipRuntimeEvidence(ctx context.Context, addr netip.Addr, order []string) openapi.IPDiagnosticRuntimeEvidence {
	family := openapi.IPDiagnosticRuntimeEvidenceFamily("ipv4")
	suffix := ""
	firewallCommand := "iptables"
	if addr.Is6() {
		family = openapi.IPDiagnosticRuntimeEvidenceFamily("ipv6")
		suffix = "v6"
		firewallCommand = "ip6tables"
	}
	resp := openapi.IPDiagnosticRuntimeEvidence{
		Family:      family,
		Memberships: []openapi.IPDiagnosticIPSetMembership{},
		Errors:      []string{},
	}

	listOut, listErr := runCommand("ipset", "list", "-n")
	if listErr != nil {
		resp.Errors = append(resp.Errors, "ipset list: "+commandErrorMessage(listOut, listErr))
		return resp
	}
	resp.IpsetAvailable = true
	existing := map[string]bool{}
	for _, name := range splitCommandLines(listOut) {
		existing[name] = true
	}

	firewallOut, firewallErr := runCommand(firewallCommand, "-w", "-t", "mangle", "-S", "PREROUTING")
	resp.FirewallAvailable = firewallErr == nil
	if firewallErr != nil {
		resp.Errors = append(resp.Errors, firewallCommand+": "+commandErrorMessage(firewallOut, firewallErr))
	}
	firewallRules := filterHRNeoRules(splitCommandLines(firewallOut))

	policyMarks := map[string]*string{}
	policies, policyErr := s.listRCIPolicies(ctx)
	resp.PolicyMarksAvailable = policyErr == nil
	if policyErr != nil {
		resp.Errors = append(resp.Errors, "RCI policies: "+policyErr.Error())
	} else {
		for _, policy := range policies {
			policyMarks[policy.Name] = policy.Mark
		}
	}

	for priority, target := range order {
		setName := target + suffix
		membership := openapi.IPDiagnosticIPSetMembership{
			Target:        target,
			SetName:       setName,
			Priority:      priority,
			Exists:        existing[setName],
			FirewallRules: firewallRulesForSet(firewallRules, setName),
			PolicyMark:    policyMarks[target],
		}
		if membership.Exists {
			_, membershipErr := runCommand("ipset", "test", setName, addr.String())
			membership.Member = membershipErr == nil
		}
		resp.Memberships = append(resp.Memberships, membership)
		if membership.Member && !resp.Matched {
			resp.Matched = true
			resp.Target = &membership.Target
			resp.SetName = &membership.SetName
			resp.PolicyMark = membership.PolicyMark
		}
	}
	return resp
}

func firewallRulesForSet(rules []string, setName string) []string {
	out := []string{}
	needle := "--match-set " + setName + " "
	for _, rule := range rules {
		if strings.Contains(rule, needle) {
			out = append(out, rule)
		}
	}
	return out
}

func domainRuleMatch(domain, rule string) (openapi.DomainDiagnosticCandidateMatchType, int, bool) {
	if domain == rule {
		return openapi.DomainDiagnosticCandidateMatchTypeExact, len(rule) + 1, true
	}
	if strings.HasSuffix(domain, "."+rule) {
		return openapi.DomainDiagnosticCandidateMatchTypeSuffix, len(rule), true
	}
	return "", 0, false
}

func diagnosticTargetOrder(hrneo openapi.HRNeoConfig, domains openapi.DomainConfig, cidr openapi.CIDRConfig) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, target := range domains.Targets {
		if target.Enabled {
			add(target.Name)
		}
	}
	for _, block := range cidr.Blocks {
		if block.Enabled {
			add(block.Target)
		}
	}
	ordered := make([]string, 0, len(names))
	used := map[string]bool{}
	for _, name := range stringSliceValue(hrneo.PolicyOrder) {
		if seen[name] && !used[name] {
			ordered = append(ordered, name)
			used[name] = true
		}
	}
	var rest []string
	for _, name := range names {
		if !used[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

func priorityMap(order []string) map[string]int {
	out := map[string]int{}
	for i, name := range order {
		out[name] = i
	}
	return out
}

func targetPriority(priority map[string]int, target string) int {
	if p, ok := priority[target]; ok {
		return p
	}
	return len(priority)
}
