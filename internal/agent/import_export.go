package agent

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
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
	data, err := exportRoutingCSV(domains, cidr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="hydraroute.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImportCSVPreview(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.ImportCSVRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := previewRoutingCSV(req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleImportCSVApply(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.ImportCSVRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
		s.writeAudit(r, "csv.import", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := previewRoutingCSV(req.Content)
	if err != nil {
		s.writeAudit(r, "csv.import", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !preview.DomainValidation.Ok || !preview.CidrValidation.Ok {
		err := fmt.Errorf("csv contains validation errors")
		s.writeAudit(r, "csv.import", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	backup, err := s.createBackup(configBackupReason("csv-import"))
	if err != nil {
		s.writeAudit(r, "csv.import", "", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.DomainConf, []byte(renderDomainConfig(preview.Domains)), 0o600); err != nil {
		s.writeAudit(r, "csv.import", "", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.CIDRList, []byte(renderCIDRConfig(preview.Cidr)), 0o600); err != nil {
		s.writeAudit(r, "csv.import", "", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := openapi.ImportCSVApplyResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.ImportCSVApplyResponseRequiredActionRestart,
		Preview:        preview,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "csv.import", "", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "csv.import", "", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleImportTextPreview(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.ImportTextRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := previewTextImport(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleImportTextApply(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.ImportTextRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
		s.writeAudit(r, "text.import", "", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := previewTextImport(req)
	if err != nil {
		s.writeAudit(r, "text.import", req.Target, false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !preview.DomainValidation.Ok || !preview.CidrValidation.Ok {
		err := fmt.Errorf("text import contains validation errors")
		s.writeAudit(r, "text.import", req.Target, false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	backup, err := s.createBackup(configBackupReason("text-import"))
	if err != nil {
		s.writeAudit(r, "text.import", req.Target, false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if req.Section == openapi.ImportTextRequestSectionDomain {
		existing, err := parseDomainConfigFile(s.cfg.DomainConf)
		if err != nil {
			s.writeAudit(r, "text.import", req.Target, false, backup.Id, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		existing.Targets = append(existing.Targets, preview.Domains.Targets...)
		if err := atomicWriteFile(s.cfg.DomainConf, []byte(renderDomainConfig(existing)), 0o600); err != nil {
			s.writeAudit(r, "text.import", req.Target, false, backup.Id, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		existing, err := parseCIDRConfigFile(s.cfg.CIDRList)
		if err != nil {
			s.writeAudit(r, "text.import", req.Target, false, backup.Id, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		existing.Blocks = append(existing.Blocks, preview.Cidr.Blocks...)
		if err := atomicWriteFile(s.cfg.CIDRList, []byte(renderCIDRConfig(existing)), 0o600); err != nil {
			s.writeAudit(r, "text.import", req.Target, false, backup.Id, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	resp := openapi.ImportCSVApplyResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.ImportCSVApplyResponseRequiredActionRestart,
		Preview:        preview,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		resp.Applied = err == nil
		if err != nil {
			s.writeAudit(r, "text.import", req.Target, false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
	}
	s.writeAudit(r, "text.import", req.Target, true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}

func previewTextImport(req openapi.ImportTextRequest) (openapi.ImportCSVPreviewResponse, error) {
	if !req.Section.Valid() {
		return openapi.ImportCSVPreviewResponse{}, fmt.Errorf("section must be domain or cidr")
	}
	if err := validateTargetName(req.Target); err != nil {
		return openapi.ImportCSVPreviewResponse{}, fmt.Errorf("target: %w", err)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	var domains openapi.DomainConfig
	var cidr openapi.CIDRConfig
	switch req.Section {
	case openapi.ImportTextRequestSectionDomain:
		target := openapi.DomainTarget{Name: req.Target, Enabled: enabled, Domains: []string{}, Geosite: []string{}}
		for _, line := range importTextLines(req.Content) {
			if strings.HasPrefix(line, "geosite:") {
				target.Geosite = append(target.Geosite, strings.TrimSpace(strings.TrimPrefix(line, "geosite:")))
			} else {
				target.Domains = append(target.Domains, line)
			}
		}
		domains.Targets = append(domains.Targets, target)
	case openapi.ImportTextRequestSectionCidr:
		block := openapi.CIDRBlock{Target: req.Target, Enabled: enabled, Entries: []string{}, Geoip: []string{}}
		for _, line := range importTextLines(req.Content) {
			if strings.HasPrefix(line, "geoip:") {
				block.Geoip = append(block.Geoip, strings.TrimSpace(strings.TrimPrefix(line, "geoip:")))
			} else {
				block.Entries = append(block.Entries, line)
			}
		}
		cidr.Blocks = append(cidr.Blocks, block)
	}
	return openapi.ImportCSVPreviewResponse{
		Domains:          domains,
		Cidr:             cidr,
		DomainValidation: validationResponse(validateDomainConfigDeep(domains)),
		CidrValidation:   validationResponse(validateCIDRConfigDeep(cidr)),
	}, nil
}

func importTextLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func exportRoutingCSV(domains openapi.DomainConfig, cidr openapi.CIDRConfig) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"section", "target", "kind", "value", "enabled", "comment"}); err != nil {
		return nil, err
	}
	for _, target := range domains.Targets {
		comment := optionalString(target.Comment)
		for _, tag := range target.Geosite {
			if err := w.Write([]string{"domain", target.Name, "geosite", tag, strconv.FormatBool(target.Enabled), comment}); err != nil {
				return nil, err
			}
		}
		for _, domain := range target.Domains {
			if err := w.Write([]string{"domain", target.Name, "domain", domain, strconv.FormatBool(target.Enabled), comment}); err != nil {
				return nil, err
			}
		}
		if !target.Enabled && len(target.Domains) == 0 && len(target.Geosite) == 0 {
			if err := w.Write([]string{"domain", target.Name, "target", "", "false", comment}); err != nil {
				return nil, err
			}
		}
	}
	for _, block := range cidr.Blocks {
		comment := optionalString(block.Comment)
		for _, tag := range block.Geoip {
			if err := w.Write([]string{"cidr", block.Target, "geoip", tag, strconv.FormatBool(block.Enabled), comment}); err != nil {
				return nil, err
			}
		}
		for _, entry := range block.Entries {
			if err := w.Write([]string{"cidr", block.Target, "cidr", entry, strconv.FormatBool(block.Enabled), comment}); err != nil {
				return nil, err
			}
		}
		if !block.Enabled && len(block.Entries) == 0 && len(block.Geoip) == 0 {
			if err := w.Write([]string{"cidr", block.Target, "target", "", "false", comment}); err != nil {
				return nil, err
			}
		}
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func previewRoutingCSV(content string) (openapi.ImportCSVPreviewResponse, error) {
	domains, cidr, err := parseRoutingCSV(content)
	if err != nil {
		return openapi.ImportCSVPreviewResponse{}, err
	}
	return openapi.ImportCSVPreviewResponse{
		Domains:          domains,
		Cidr:             cidr,
		DomainValidation: validationResponse(validateDomainConfigDeep(domains)),
		CidrValidation:   validationResponse(validateCIDRConfigDeep(cidr)),
	}, nil
}

func parseRoutingCSV(content string) (openapi.DomainConfig, openapi.CIDRConfig, error) {
	r := csv.NewReader(strings.NewReader(content))
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return openapi.DomainConfig{}, openapi.CIDRConfig{}, err
	}
	if len(rows) == 0 {
		return openapi.DomainConfig{}, openapi.CIDRConfig{}, nil
	}
	if len(rows[0]) >= 3 && strings.EqualFold(rows[0][0], "section") {
		rows = rows[1:]
	}

	domainTargets := map[string]*openapi.DomainTarget{}
	cidrBlocks := map[string]*openapi.CIDRBlock{}
	var domainOrder []string
	var cidrOrder []string
	for i, row := range rows {
		if len(row) < 5 {
			return openapi.DomainConfig{}, openapi.CIDRConfig{}, fmt.Errorf("row %d must have at least 5 columns", i+1)
		}
		section := strings.ToLower(strings.TrimSpace(row[0]))
		target := strings.TrimSpace(row[1])
		kind := strings.ToLower(strings.TrimSpace(row[2]))
		value := strings.TrimSpace(row[3])
		enabled, err := strconv.ParseBool(strings.TrimSpace(row[4]))
		if err != nil {
			return openapi.DomainConfig{}, openapi.CIDRConfig{}, fmt.Errorf("row %d enabled must be boolean", i+1)
		}
		comment := ""
		if len(row) > 5 {
			comment = strings.TrimSpace(row[5])
		}
		switch section {
		case "domain":
			item := domainTargets[target]
			if item == nil {
				domainOrder = append(domainOrder, target)
				item = &openapi.DomainTarget{Name: target, Enabled: enabled, Domains: []string{}, Geosite: []string{}}
				domainTargets[target] = item
			}
			item.Enabled = item.Enabled || enabled
			setCommentIfPresent(&item.Comment, comment)
			switch kind {
			case "domain":
				item.Domains = append(item.Domains, value)
			case "geosite":
				item.Geosite = append(item.Geosite, value)
			case "target":
			default:
				return openapi.DomainConfig{}, openapi.CIDRConfig{}, fmt.Errorf("row %d has unsupported domain kind %q", i+1, kind)
			}
		case "cidr":
			item := cidrBlocks[target]
			if item == nil {
				cidrOrder = append(cidrOrder, target)
				item = &openapi.CIDRBlock{Target: target, Enabled: enabled, Entries: []string{}, Geoip: []string{}}
				cidrBlocks[target] = item
			}
			item.Enabled = item.Enabled || enabled
			setCommentIfPresent(&item.Comment, comment)
			switch kind {
			case "cidr":
				item.Entries = append(item.Entries, value)
			case "geoip":
				item.Geoip = append(item.Geoip, value)
			case "target":
			default:
				return openapi.DomainConfig{}, openapi.CIDRConfig{}, fmt.Errorf("row %d has unsupported cidr kind %q", i+1, kind)
			}
		default:
			return openapi.DomainConfig{}, openapi.CIDRConfig{}, fmt.Errorf("row %d has unsupported section %q", i+1, section)
		}
	}

	domains := openapi.DomainConfig{}
	for _, target := range domainOrder {
		domains.Targets = append(domains.Targets, *domainTargets[target])
	}
	cidr := openapi.CIDRConfig{}
	for _, target := range cidrOrder {
		cidr.Blocks = append(cidr.Blocks, *cidrBlocks[target])
	}
	return domains, cidr, nil
}

func setCommentIfPresent(dst **string, comment string) {
	if comment == "" || *dst != nil {
		return
	}
	*dst = &comment
}

func optionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
