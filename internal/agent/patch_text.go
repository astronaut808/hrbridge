package agent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type duplicateRuleGroupError struct {
	group string
}

func (e duplicateRuleGroupError) Error() string {
	if e.group == "" {
		return "rule already exists in ungrouped rules"
	}
	return fmt.Sprintf("rule already exists in group %q", e.group)
}

func patchDomainConfigText(content, target string, req openapi.DomainRulePatchRequest, add bool) (string, error) {
	token, err := domainPatchToken(req)
	if err != nil {
		return "", err
	}
	comment, err := patchComment(req.Comment)
	if err != nil {
		return "", err
	}
	lines, trailing := splitConfigLines(content)
	match := func(value string) bool { return strings.EqualFold(strings.TrimSpace(value), token) }
	firstTarget := -1
	firstGroupTarget := -1
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
		inRequestedGroup := comment == nil || commentMatchesLineGroup(lines, i, *comment)
		if inRequestedGroup && firstGroupTarget < 0 {
			firstGroupTarget = i
		}
		var kept []string
		lineFound := false
		for _, entry := range strings.Split(left, ",") {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			if match(trimmed) {
				if add && comment != nil && !inRequestedGroup {
					return "", duplicateRuleGroupError{group: commentGroupName(lines, i)}
				}
				if comment != nil && !inRequestedGroup {
					kept = append(kept, entry)
					continue
				}
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
				start := domainLineBlockStart(lines, i)
				lines = append(lines[:start], lines[i+1:]...)
				i = start - 1
				continue
			}
			lines[i] = strings.Join(kept, ",") + line[strings.LastIndex(line, "/"):]
		}
	}
	if add && !found {
		targetLine := firstTarget
		if comment != nil {
			targetLine = firstGroupTarget
		}
		if targetLine >= 0 {
			slash := strings.LastIndex(lines[targetLine], "/")
			lines[targetLine] = lines[targetLine][:slash] + "," + token + lines[targetLine][slash:]
		} else {
			if comment != nil {
				lines = append(lines, "##"+*comment)
			}
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
	comment, err := patchComment(req.Comment)
	if err != nil {
		return "", err
	}
	lines, trailing := splitConfigLines(content)
	activeTarget := ""
	activeGroupMatches := false
	activeHeaderIndex := -1
	insertAt := -1
	found := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "" || strings.HasPrefix(line, "##"):
			if activeTarget == target && activeGroupMatches && insertAt < 0 {
				insertAt = i
			}
			activeTarget = ""
			activeGroupMatches = false
			activeHeaderIndex = -1
		case strings.HasPrefix(line, "#/"):
			if activeTarget == target && activeGroupMatches && insertAt < 0 {
				insertAt = i
			}
			activeTarget = ""
			activeGroupMatches = false
			activeHeaderIndex = -1
		case strings.HasPrefix(line, "/"):
			if activeTarget == target && activeGroupMatches && insertAt < 0 {
				insertAt = i
			}
			activeTarget = strings.TrimSpace(strings.TrimPrefix(line, "/"))
			activeGroupMatches = comment == nil || commentMatchesLineGroup(lines, i, *comment)
			activeHeaderIndex = i
		case activeTarget == target && strings.EqualFold(line, token):
			if add && comment != nil && !activeGroupMatches {
				return "", duplicateRuleGroupError{group: commentGroupName(lines, activeHeaderIndex)}
			}
			if comment != nil && !activeGroupMatches {
				continue
			}
			found = true
			if !add {
				lines = append(lines[:i], lines[i+1:]...)
				i--
			}
		}
		if activeTarget == target && activeGroupMatches {
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
			if comment != nil {
				lines = append(lines, "##"+*comment)
			}
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
		start := cidrBlockStart(lines, i)
		lines = append(lines[:start], lines[end:]...)
		i = start
	}
	return lines
}

func domainLineBlockStart(lines []string, lineIndex int) int {
	return commentBlockStart(lines, lineIndex)
}

func cidrBlockStart(lines []string, lineIndex int) int {
	return commentBlockStart(lines, lineIndex)
}

func commentBlockStart(lines []string, lineIndex int) int {
	start := lineIndex
	for start > 0 {
		prev := strings.TrimSpace(lines[start-1])
		if !strings.HasPrefix(prev, "##") {
			break
		}
		start--
	}
	return start
}

func commentMatchesLineGroup(lines []string, lineIndex int, comment string) bool {
	var comments []string
	for i := lineIndex - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "##") {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "##"))
		if item != "" {
			comments = append([]string{item}, comments...)
		}
	}
	for _, item := range comments {
		if strings.EqualFold(strings.TrimSpace(item), comment) {
			return true
		}
	}
	return strings.EqualFold(strings.Join(comments, "\n"), comment)
}

func commentGroupName(lines []string, lineIndex int) string {
	var comments []string
	for i := lineIndex - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "##") {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "##"))
		if item != "" {
			comments = append([]string{item}, comments...)
		}
	}
	return strings.Join(comments, "\n")
}

func patchComment(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	comment := strings.TrimSpace(*value)
	if comment == "" {
		return nil, nil
	}
	if hasLineBreak(comment) {
		return nil, errors.New("comment must not contain line breaks")
	}
	return &comment, nil
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
