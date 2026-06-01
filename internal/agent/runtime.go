package agent

import (
	"net/http"
	"sort"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleRuntimeIPSets(w http.ResponseWriter, r *http.Request) {
	out, err := runCommand("ipset", "list", "-n")
	sets := splitCommandLines(out)
	setMap := map[string]bool{}
	for _, name := range sets {
		setMap[name] = true
	}
	resp := openapi.RuntimeIPSetsResponse{
		Available:      err == nil,
		Sets:           sets,
		ReferencedSets: s.referencedIPSets(setMap),
	}
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		resp.Error = &msg
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRuntimeFirewall(w http.ResponseWriter, r *http.Request) {
	v4, err4 := runCommand("iptables", "-w", "-t", "mangle", "-S", "PREROUTING")
	v6, err6 := runCommand("ip6tables", "-w", "-t", "mangle", "-S", "PREROUTING")
	resp := openapi.RuntimeFirewallResponse{
		Ipv4Available: err4 == nil,
		Ipv6Available: err6 == nil,
		Ipv4Rules:     filterHRNeoRules(splitCommandLines(v4)),
		Ipv6Rules:     filterHRNeoRules(splitCommandLines(v6)),
	}
	if err4 != nil {
		msg := commandErrorMessage(v4, err4)
		resp.Ipv4Error = &msg
	}
	if err6 != nil {
		msg := commandErrorMessage(v6, err6)
		resp.Ipv6Error = &msg
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRuntimeDirectRoutes(w http.ResponseWriter, r *http.Request) {
	ifaces, err := listSystemInterfaces()
	if err != nil {
		ifaces = []openapi.InterfaceInfo{}
	}
	v4, err4 := runCommand("ip", "rule", "show")
	v6, err6 := runCommand("ip", "-6", "rule", "show")
	resp := openapi.RuntimeDirectRoutesResponse{
		Interfaces: ifaces,
		Ipv4Rules:  splitCommandLines(v4),
		Ipv6Rules:  splitCommandLines(v6),
	}
	if err4 != nil {
		msg := commandErrorMessage(v4, err4)
		resp.Ipv4Error = &msg
	}
	if err6 != nil {
		msg := commandErrorMessage(v6, err6)
		resp.Ipv6Error = &msg
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) referencedIPSets(existing map[string]bool) []openapi.RuntimeIPSetInfo {
	inventory, err := s.targetInventory()
	if err != nil {
		return []openapi.RuntimeIPSetInfo{}
	}
	var out []openapi.RuntimeIPSetInfo
	seen := map[string]bool{}
	for _, target := range inventory.Targets {
		for _, name := range []string{target.Name, target.Name + "v6"} {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, openapi.RuntimeIPSetInfo{Name: name, Exists: existing[name]})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func splitCommandLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func filterHRNeoRules(lines []string) []string {
	var out []string
	for _, line := range lines {
		if strings.Contains(line, "--match-set") || strings.Contains(line, "NFQUEUE") {
			out = append(out, line)
		}
	}
	return out
}

func commandErrorMessage(out string, err error) string {
	msg := strings.TrimSpace(out)
	if msg == "" {
		msg = err.Error()
	}
	return msg
}
