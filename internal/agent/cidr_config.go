package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func parseCIDRConfigFile(path string) (openapi.CIDRConfig, error) {
	f, err := os.Open(path) // #nosec G304 -- CIDR config path comes from root-owned HydraBridge configuration
	if err != nil {
		return openapi.CIDRConfig{}, err
	}
	defer func() { _ = f.Close() }()
	return parseCIDRConfig(f)
}

func parseCIDRConfig(r io.Reader) (openapi.CIDRConfig, error) {
	var cfg openapi.CIDRConfig
	var pendingComment []string
	var current *openapi.CIDRBlock

	flush := func() {
		if current == nil {
			return
		}
		cfg.Blocks = append(cfg.Blocks, *current)
		current = nil
	}

	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			flush()
			pendingComment = nil
			continue
		}
		if strings.HasPrefix(line, "##") {
			flush()
			comment := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			if comment != "" {
				pendingComment = append(pendingComment, comment)
			}
			continue
		}
		if strings.HasPrefix(line, "#/") {
			flush()
			target := strings.TrimSpace(strings.TrimPrefix(line, "#/"))
			current = &openapi.CIDRBlock{
				Target:  target,
				Enabled: false,
				Entries: []string{},
				Geoip:   []string{},
			}
			setCIDRPendingComment(current, pendingComment)
			pendingComment = nil
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "/") {
			flush()
			target := strings.TrimSpace(strings.TrimPrefix(line, "/"))
			current = &openapi.CIDRBlock{
				Target:  target,
				Enabled: true,
				Entries: []string{},
				Geoip:   []string{},
			}
			setCIDRPendingComment(current, pendingComment)
			pendingComment = nil
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "geoip:") {
			tag := strings.TrimSpace(strings.TrimPrefix(line, "geoip:"))
			if tag != "" {
				current.Geoip = append(current.Geoip, tag)
			}
			continue
		}
		current.Entries = append(current.Entries, line)
	}
	if err := sc.Err(); err != nil {
		return openapi.CIDRConfig{}, err
	}
	flush()
	return cfg, nil
}

func renderCIDRConfig(cfg openapi.CIDRConfig) string {
	var b strings.Builder
	for i, block := range cfg.Blocks {
		if i > 0 {
			b.WriteByte('\n')
		}
		if block.Comment != nil && strings.TrimSpace(*block.Comment) != "" {
			for _, line := range strings.Split(*block.Comment, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(&b, "##%s\n", line)
				}
			}
		}
		target := strings.TrimSpace(block.Target)
		if target == "" {
			continue
		}
		if block.Enabled {
			fmt.Fprintf(&b, "/%s\n", target)
		} else {
			fmt.Fprintf(&b, "#/%s\n", target)
		}
		for _, tag := range block.Geoip {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				fmt.Fprintf(&b, "geoip:%s\n", tag)
			}
		}
		for _, entry := range block.Entries {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				fmt.Fprintf(&b, "%s\n", entry)
			}
		}
	}
	return b.String()
}

func setCIDRPendingComment(block *openapi.CIDRBlock, comments []string) {
	if len(comments) == 0 {
		return
	}
	comment := strings.Join(comments, "\n")
	block.Comment = &comment
}
