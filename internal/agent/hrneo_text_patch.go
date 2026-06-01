package agent

import (
	"fmt"
	"strings"
)

func replaceHRNeoConfigValue(content, key, value string) string {
	lines, trailingNewline := splitTextLines(content)
	replacement := key + "=" + value
	out := make([]string, 0, len(lines)+1)
	replaced := false

	for _, line := range lines {
		if hrneoConfigLineKey(line) != key {
			out = append(out, line)
			continue
		}
		if !replaced {
			out = append(out, replacement)
			replaced = true
		}
	}
	if !replaced {
		out = append(out, replacement)
		trailingNewline = true
	}
	return joinTextLines(out, trailingNewline)
}

func appendHRNeoRepeatValue(content, key, value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, line := range strings.Split(content, "\n") {
		lineKey, lineValue, ok := parseHRNeoConfigTextLine(line)
		if ok && lineKey == key && lineValue == value {
			return content, false
		}
	}

	lines, trailingNewline := splitTextLines(content)
	lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	return joinTextLines(lines, trailingNewline || len(lines) > 0), true
}

func hrneoConfigLineKey(line string) string {
	key, _, ok := parseHRNeoConfigTextLine(line)
	if !ok {
		return ""
	}
	return key
}

func parseHRNeoConfigTextLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(value), true
}

func splitTextLines(content string) ([]string, bool) {
	if content == "" {
		return []string{}, false
	}
	trailingNewline := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if trailingNewline {
		lines = lines[:len(lines)-1]
	}
	return lines, trailingNewline
}

func joinTextLines(lines []string, trailingNewline bool) string {
	content := strings.Join(lines, "\n")
	if trailingNewline {
		content += "\n"
	}
	return content
}
