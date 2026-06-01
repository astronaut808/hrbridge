package agent

import (
	"net/http"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleGroupedDomains(w http.ResponseWriter, r *http.Request) {
	cfg, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.GroupedDomainViewResponse{
		Path:    s.cfg.DomainConf,
		Targets: groupDomainTargets(cfg),
	})
}

func (s *Server) handleGroupedCIDR(w http.ResponseWriter, r *http.Request) {
	cfg, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.GroupedCIDRViewResponse{
		Path:    s.cfg.CIDRList,
		Targets: groupCIDRTargets(cfg),
	})
}

func groupDomainTargets(cfg openapi.DomainConfig) []openapi.GroupedDomainTarget {
	var out []openapi.GroupedDomainTarget
	index := map[string]int{}
	for _, group := range cfg.Targets {
		name := strings.TrimSpace(group.Name)
		i, ok := index[name]
		if !ok {
			i = len(out)
			index[name] = i
			out = append(out, openapi.GroupedDomainTarget{
				Name:    name,
				Domains: []string{},
				Geosite: []string{},
				Groups:  []openapi.DomainTarget{},
			})
		}
		item := &out[i]
		item.Enabled = item.Enabled || group.Enabled
		item.Domains = appendUniqueFold(item.Domains, group.Domains...)
		item.Geosite = appendUniqueFold(item.Geosite, group.Geosite...)
		item.Groups = append(item.Groups, group)
	}
	return out
}

func groupCIDRTargets(cfg openapi.CIDRConfig) []openapi.GroupedCIDRTarget {
	var out []openapi.GroupedCIDRTarget
	index := map[string]int{}
	for _, block := range cfg.Blocks {
		target := strings.TrimSpace(block.Target)
		i, ok := index[target]
		if !ok {
			i = len(out)
			index[target] = i
			out = append(out, openapi.GroupedCIDRTarget{
				Target:  target,
				Entries: []string{},
				Geoip:   []string{},
				Blocks:  []openapi.CIDRBlock{},
			})
		}
		item := &out[i]
		item.Enabled = item.Enabled || block.Enabled
		item.Entries = appendUniqueFold(item.Entries, block.Entries...)
		item.Geoip = appendUniqueFold(item.Geoip, block.Geoip...)
		item.Blocks = append(item.Blocks, block)
	}
	return out
}

func appendUniqueFold(dst []string, values ...string) []string {
	for _, value := range values {
		found := false
		for _, existing := range dst {
			if strings.EqualFold(existing, value) {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, value)
		}
	}
	return dst
}
