package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type targetAccumulator struct {
	info       openapi.TargetInfo
	sourceSet  map[openapi.TargetInfoSources]bool
	enabledSet bool
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	inventory, err := s.targetInventory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, inventory)
}

func (s *Server) handleTargetInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := listSystemInterfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.InterfacesResponse{Interfaces: interfaces})
}

func (s *Server) handleTargetPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.listRCIPolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.PoliciesResponse{Policies: policies})
}

func (s *Server) handleCreateTargetPolicy(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PolicyMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "targets.policy.create", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateTargetName(req.Name); err != nil {
		s.writeAudit(r, "targets.policy.create", req.Name, false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	save := true
	if req.Save != nil {
		save = *req.Save
	}
	command := "ip policy " + strings.TrimSpace(req.Name)
	output, err := s.rciExecuteCommands(r.Context(), []string{command}, save)
	if err != nil {
		s.writeAudit(r, "targets.policy.create", req.Name, false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r, "targets.policy.create", req.Name, true, "", nil)
	writeJSON(w, http.StatusOK, openapi.PolicyMutationResponse{
		Ok:      true,
		Command: command,
		Saved:   save,
		Output:  &output,
	})
}

func (s *Server) handleDeleteTargetPolicy(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := validateTargetName(name); err != nil {
		s.writeAudit(r, "targets.policy.delete", name, false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	command := "no ip policy " + name
	output, err := s.rciExecuteCommands(r.Context(), []string{command}, true)
	if err != nil {
		s.writeAudit(r, "targets.policy.delete", name, false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r, "targets.policy.delete", name, true, "", nil)
	writeJSON(w, http.StatusOK, openapi.PolicyMutationResponse{
		Ok:      true,
		Command: command,
		Saved:   true,
		Output:  &output,
	})
}

func (s *Server) handleGetTargetOrder(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.TargetOrderResponse{Order: stringSliceValue(cfg.PolicyOrder)})
}

func (s *Server) handlePutTargetOrder(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.PutTargetOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAudit(r, "targets.order.write", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateTargetOrder(req.Order); err != nil {
		s.writeAudit(r, "targets.order.write", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	_, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil {
		s.writeAudit(r, "targets.order.write", "", false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	content, err := os.ReadFile(s.cfg.HRNeoConf)
	if err != nil {
		s.writeAudit(r, "targets.order.write", "", false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	backup, err := s.createBackup(configBackupReason("targets-order"))
	if err != nil {
		s.writeAudit(r, "targets.order.write", "", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	patched := replaceHRNeoConfigValue(string(content), "PolicyOrder", strings.Join(req.Order, ","))
	if err := atomicWriteFile(s.cfg.HRNeoConf, []byte(patched), 0o600); err != nil {
		s.writeAudit(r, "targets.order.write", "", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}

	resp := openapi.PutConfigResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.PutConfigResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "targets.order.write", "", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "targets.order.write", "", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) targetInventory() (openapi.TargetInventoryResponse, error) {
	targets := map[string]*targetAccumulator{}

	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.TargetInventoryResponse{}, err
	}
	for _, target := range domains.Targets {
		acc := ensureTarget(targets, target.Name)
		addTargetSource(acc, openapi.TargetInfoSourcesDomain)
		acc.info.DomainRules += len(target.Domains) + len(target.Geosite)
		mergeEnabled(acc, target.Enabled)
	}

	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil && !os.IsNotExist(err) {
		return openapi.TargetInventoryResponse{}, err
	}
	for _, block := range cidr.Blocks {
		acc := ensureTarget(targets, block.Target)
		addTargetSource(acc, openapi.TargetInfoSourcesCidr)
		acc.info.CidrRules += len(block.Entries) + len(block.Geoip)
		mergeEnabled(acc, block.Enabled)
	}

	hrneo, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.TargetInventoryResponse{}, err
	}
	order := stringSliceValue(hrneo.PolicyOrder)
	for _, name := range order {
		acc := ensureTarget(targets, name)
		addTargetSource(acc, openapi.TargetInfoSourcesPolicyOrder)
	}

	out := openapi.TargetInventoryResponse{Order: order}
	for _, acc := range targets {
		acc.info.Sources = sourceList(acc.sourceSet)
		acc.info.Type, acc.info.InterfaceState = classifyTarget(acc.info.Name)
		out.Targets = append(out.Targets, acc.info)
	}
	sort.Slice(out.Targets, func(i, j int) bool {
		return strings.ToLower(out.Targets[i].Name) < strings.ToLower(out.Targets[j].Name)
	})
	return out, nil
}

func ensureTarget(targets map[string]*targetAccumulator, name string) *targetAccumulator {
	name = strings.TrimSpace(name)
	if targets[name] == nil {
		enabled := false
		targets[name] = &targetAccumulator{
			info: openapi.TargetInfo{
				Name:    name,
				Type:    openapi.TargetInfoTypeUnknown,
				Enabled: &enabled,
			},
			sourceSet: map[openapi.TargetInfoSources]bool{},
		}
	}
	return targets[name]
}

func addTargetSource(acc *targetAccumulator, source openapi.TargetInfoSources) {
	acc.sourceSet[source] = true
}

func mergeEnabled(acc *targetAccumulator, enabled bool) {
	if enabled {
		v := true
		acc.info.Enabled = &v
		acc.enabledSet = true
		return
	}
	if !acc.enabledSet && acc.info.Enabled == nil {
		v := false
		acc.info.Enabled = &v
	}
}

func sourceList(values map[openapi.TargetInfoSources]bool) []openapi.TargetInfoSources {
	order := []openapi.TargetInfoSources{
		openapi.TargetInfoSourcesDomain,
		openapi.TargetInfoSourcesCidr,
		openapi.TargetInfoSourcesPolicyOrder,
	}
	out := make([]openapi.TargetInfoSources, 0, len(values))
	for _, source := range order {
		if values[source] {
			out = append(out, source)
		}
	}
	return out
}

func classifyTarget(name string) (openapi.TargetInfoType, *openapi.TargetInfoInterfaceState) {
	if name == "" {
		return openapi.TargetInfoTypeUnknown, nil
	}
	data, err := readInterfaceOperstate(name)
	if err == nil {
		state := openapi.TargetInfoInterfaceState(strings.TrimSpace(string(data)))
		if !state.Valid() {
			state = openapi.TargetInfoInterfaceStateUnknown
		}
		return openapi.TargetInfoTypeInterface, &state
	}
	if os.IsNotExist(err) {
		return openapi.TargetInfoTypePolicy, nil
	}
	return openapi.TargetInfoTypeUnknown, nil
}

func listSystemInterfaces() ([]openapi.InterfaceInfo, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		if os.IsNotExist(err) {
			return []openapi.InterfaceInfo{}, nil
		}
		return nil, err
	}
	out := make([]openapi.InterfaceInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == "" {
			continue
		}
		state := readInterfaceState(entry.Name())
		addresses := interfaceAddresses(entry.Name())
		out = append(out, openapi.InterfaceInfo{
			Name:      entry.Name(),
			State:     state,
			Addresses: &addresses,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func interfaceAddresses(name string) []string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return []string{}
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	sort.Strings(out)
	return out
}

func readInterfaceState(name string) openapi.InterfaceInfoState {
	data, err := readInterfaceOperstate(name)
	if err != nil {
		return openapi.InterfaceInfoStateUnknown
	}
	state := openapi.InterfaceInfoState(strings.TrimSpace(string(data)))
	if !state.Valid() {
		return openapi.InterfaceInfoStateUnknown
	}
	return state
}

func interfaceStatePath(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid interface name")
	}
	return filepath.Join("/sys/class/net", name, "operstate"), nil
}

func readInterfaceOperstate(name string) ([]byte, error) {
	if _, err := interfaceStatePath(name); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot("/sys/class/net")
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(filepath.Join(name, "operstate"))
}

func (s *Server) listRCIPolicies(ctx context.Context) ([]openapi.PolicyInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.RCIURL, "/")+"/rci/show/ip/policy/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RCI status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var raw map[string]struct {
		Mark *string `json:"mark"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	policies := make([]openapi.PolicyInfo, 0, len(raw))
	for name, policy := range raw {
		policies = append(policies, openapi.PolicyInfo{
			Name: name,
			Mark: policy.Mark,
		})
	}
	sort.Slice(policies, func(i, j int) bool {
		return strings.ToLower(policies[i].Name) < strings.ToLower(policies[j].Name)
	})
	return policies, nil
}

func (s *Server) rciExecuteCommands(ctx context.Context, commands []string, save bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	payload := make([]map[string]string, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			payload = append(payload, map[string]string{"parse": command})
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	output, err := s.rciPost(ctx, "/rci/", body)
	if err != nil {
		return output, err
	}
	if save {
		saveOut, err := s.rciPost(ctx, "/rci/", []byte(`{"system":{"configuration":{"save":true}}}`))
		if saveOut != "" {
			if output != "" {
				output += "\n"
			}
			output += saveOut
		}
		if err != nil {
			return output, err
		}
	}
	return output, nil
}

func (s *Server) rciPost(ctx context.Context, path string, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.RCIURL, "/")+path, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	output := string(data)
	if resp.StatusCode != http.StatusOK {
		return output, fmt.Errorf("RCI status: %d", resp.StatusCode)
	}
	return output, nil
}

func stringSliceValue(values *[]string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string(nil), *values...)
}

func validateTargetOrder(order []string) error {
	seen := map[string]bool{}
	for i, target := range order {
		if err := validateTargetName(target); err != nil {
			return err
		}
		key := strings.ToLower(strings.TrimSpace(target))
		if seen[key] {
			return errors.New("order contains duplicate target: " + target)
		}
		seen[key] = true
		order[i] = strings.TrimSpace(target)
	}
	return nil
}
