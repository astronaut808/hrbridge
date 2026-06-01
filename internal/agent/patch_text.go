package agent

import (
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func patchDomainConfigText(content, target string, req openapi.DomainRulePatchRequest, add bool) (string, error) {
	token, err := domainPatchToken(req)
	if err != nil {
		return "", err
	}
	lines, trailing := splitConfigLines(content)
	match := func(value string) bool { return strings.EqualFold(strings.TrimSpace(value), token) }
	firstTarget := -1
	found := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		left, name, ok := splitActiveDomainLine(line)
		if !ok || name != target {
			continue
		}
		if firstTarget < 0 {
			firstTarget = i
		}
		var kept []string
		lineFound := false
		for _, entry := range strings.Split(left, ",") {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			if match(trimmed) {
				found = true
				lineFound = true
				if !add {
					continue
				}
			}
			kept = append(kept, entry)
		}
		if !add && lineFound {
			if len(kept) == 0 {
				lines = append(lines[:i], lines[i+1:]...)
				i--
				continue
			}
			lines[i] = strings.Join(kept, ",") + line[strings.LastIndex(line, "/"):]
		}
	}
	if add && !found {
		if firstTarget >= 0 {
			slash := strings.LastIndex(lines[firstTarget], "/")
			lines[firstTarget] = lines[firstTarget][:slash] + "," + token + lines[firstTarget][slash:]
		} else {
			lines = append(lines, token+"/"+target)
		}
	}
	return validatePatchedDomainText(joinConfigLines(lines, trailing))
}

func patchCIDRConfigText(content, target string, req openapi.CIDRRulePatchRequest, add bool) (string, error) {
	token, err := cidrPatchToken(req)
	if err != nil {
		return "", err
	}
	lines, trailing := splitConfigLines(content)
	activeTarget := ""
	insertAt := -1
	found := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "" || strings.HasPrefix(line, "##"):
			if activeTarget == target && insertAt < 0 {
				insertAt = i
			}
			activeTarget = ""
		case strings.HasPrefix(line, "#/"):
			if activeTarget == target && insertAt < 0 {
				insertAt = i
			}
			activeTarget = ""
		case strings.HasPrefix(line, "/"):
			if activeTarget == target && insertAt < 0 {
				insertAt = i
			}
			activeTarget = strings.TrimSpace(strings.TrimPrefix(line, "/"))
		case activeTarget == target && strings.EqualFold(line, token):
			found = true
			if !add {
				lines = append(lines[:i], lines[i+1:]...)
				i--
			}
		}
		if activeTarget == target {
			insertAt = i + 1
		}
	}
	if !add && found {
		lines = removeEmptyCIDRBlocks(lines, target)
	}
	if add && !found {
		if insertAt >= 0 {
			lines = append(lines, "")
			copy(lines[insertAt+1:], lines[insertAt:])
			lines[insertAt] = token
		} else {
			lines = append(lines, "/"+target, token)
		}
	}
	return validatePatchedCIDRText(joinConfigLines(lines, trailing))
}

func removeEmptyCIDRBlocks(lines []string, target string) []string {
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) != "/"+target {
			i++
			continue
		}
		end := i + 1
		for end < len(lines) {
			line := strings.TrimSpace(lines[end])
			if line == "" || strings.HasPrefix(line, "##") || strings.HasPrefix(line, "/") || strings.HasPrefix(line, "#/") {
				break
			}
			end++
		}
		if end > i+1 {
			i = end
			continue
		}
		start := i
		lines = append(lines[:start], lines[end:]...)
		i = start
	}
	return lines
}

func domainPatchToken(req openapi.DomainRulePatchRequest) (string, error) {
	if !req.Kind.Valid() {
		return "", errInvalidRuleKind
	}
	value := strings.TrimSpace(req.Value)
	if req.Kind == openapi.DomainRulePatchRequestKindDomain {
		if err := validateDomainValue(value); err != nil {
			return "", err
		}
		return strings.ToLower(value), nil
	}
	if err := validateTagValue(value); err != nil {
		return "", err
	}
	return "geosite:" + value, nil
}

func cidrPatchToken(req openapi.CIDRRulePatchRequest) (string, error) {
	if !req.Kind.Valid() {
		return "", errInvalidRuleKind
	}
	value := strings.TrimSpace(req.Value)
	if req.Kind == openapi.CIDRRulePatchRequestKindCidr {
		if err := validateIPOrPrefix(value); err != nil {
			return "", err
		}
		return value, nil
	}
	if err := validateTagValue(value); err != nil {
		return "", err
	}
	return "geoip:" + value, nil
}

func splitActiveDomainLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	slash := strings.LastIndex(line, "/")
	if slash < 0 {
		return "", "", false
	}
	target := strings.TrimSpace(line[slash+1:])
	if comma := strings.IndexByte(target, ','); comma >= 0 {
		target = strings.TrimSpace(target[:comma])
	}
	return line[:slash], target, target != ""
}

func validatePatchedDomainText(content string) (string, error) {
	if _, err := parseDomainConfig(strings.NewReader(content)); err != nil {
		return "", err
	}
	return content, nil
}

func validatePatchedCIDRText(content string) (string, error) {
	if _, err := parseCIDRConfig(strings.NewReader(content)); err != nil {
		return "", err
	}
	return content, nil
}

func splitConfigLines(content string) ([]string, bool) {
	trailing := strings.HasSuffix(content, "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}, trailing
	}
	return strings.Split(content, "\n"), trailing
}

func joinConfigLines(lines []string, trailing bool) string {
	if len(lines) == 0 {
		return ""
	}
	content := strings.Join(lines, "\n")
	if trailing {
		content += "\n"
	}
	return content
}
