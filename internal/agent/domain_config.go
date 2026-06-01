package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func parseDomainConfigFile(path string) (openapi.DomainConfig, error) {
	f, err := os.Open(path) // #nosec G304 -- domain config path comes from root-owned HydraBridge configuration
	if err != nil {
		return openapi.DomainConfig{}, err
	}
	defer func() { _ = f.Close() }()
	return parseDomainConfig(f)
}

func parseDomainConfig(r io.Reader) (openapi.DomainConfig, error) {
	var cfg openapi.DomainConfig
	var pendingComment []string
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			pendingComment = nil
			continue
		}
		if strings.HasPrefix(line, "##") {
			comment := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			if comment != "" {
				pendingComment = append(pendingComment, comment)
			}
			continue
		}
		if strings.HasPrefix(line, "#/") {
			target := strings.TrimSpace(strings.TrimPrefix(line, "#/"))
			if target != "" {
				t := openapi.DomainTarget{
					Name:    target,
					Enabled: false,
					Domains: []string{},
					Geosite: []string{},
				}
				setPendingComment(&t, pendingComment)
				cfg.Targets = append(cfg.Targets, t)
				pendingComment = nil
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		slash := strings.LastIndex(line, "/")
		if slash < 0 {
			continue
		}
		left, target := line[:slash], line[slash+1:]
		target = strings.TrimSpace(target)
		if comma := strings.IndexByte(target, ','); comma >= 0 {
			target = strings.TrimSpace(target[:comma])
		}
		if target == "" {
			continue
		}

		t := openapi.DomainTarget{
			Name:    target,
			Enabled: true,
			Domains: []string{},
			Geosite: []string{},
		}
		setPendingComment(&t, pendingComment)
		for _, token := range strings.Split(left, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if strings.HasPrefix(token, "geosite:") {
				tag := strings.TrimSpace(strings.TrimPrefix(token, "geosite:"))
				if tag != "" {
					t.Geosite = append(t.Geosite, tag)
				}
				continue
			}
			t.Domains = append(t.Domains, strings.ToLower(token))
		}
		cfg.Targets = append(cfg.Targets, t)
		pendingComment = nil
	}
	if err := sc.Err(); err != nil {
		return openapi.DomainConfig{}, err
	}
	return cfg, nil
}

func renderDomainConfig(cfg openapi.DomainConfig) string {
	var b strings.Builder
	for i, target := range cfg.Targets {
		if i > 0 {
			b.WriteByte('\n')
		}
		if target.Comment != nil && strings.TrimSpace(*target.Comment) != "" {
			for _, line := range strings.Split(*target.Comment, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(&b, "##%s\n", line)
				}
			}
		}
		if !target.Enabled {
			fmt.Fprintf(&b, "#/%s\n", target.Name)
			continue
		}
		var entries []string
		for _, tag := range target.Geosite {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				entries = append(entries, "geosite:"+tag)
			}
		}
		for _, domain := range target.Domains {
			domain = strings.ToLower(strings.TrimSpace(domain))
			if domain != "" {
				entries = append(entries, domain)
			}
		}
		if len(entries) == 0 || strings.TrimSpace(target.Name) == "" {
			continue
		}
		fmt.Fprintf(&b, "%s/%s\n", strings.Join(entries, ","), strings.TrimSpace(target.Name))
	}
	return b.String()
}

func setPendingComment(target *openapi.DomainTarget, comments []string) {
	if len(comments) == 0 {
		return
	}
	comment := strings.Join(comments, "\n")
	target.Comment = &comment
}
