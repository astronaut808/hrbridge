package agent

import (
	"strings"
	"testing"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func TestPatchDomainConfigTextPreservesUntouchedLines(t *testing.T) {
	before := strings.Join([]string{
		"## Finland primary",
		"example.com, keep-format.example/Finland",
		"",
		"## Finland secondary",
		"geosite:google,fbcdn.net/Finland",
		"# untouched comment",
		"vk.com/Russia",
		"",
	}, "\n")
	add := openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "openai.com"}
	afterAdd, err := patchDomainConfigText(before, "Finland", add, true)
	if err != nil {
		t.Fatal(err)
	}
	wantAdd := strings.Replace(before, "example.com, keep-format.example/Finland", "example.com, keep-format.example,openai.com/Finland", 1)
	if afterAdd != wantAdd {
		t.Fatalf("unexpected add diff:\n--- want\n%s\n--- got\n%s", wantAdd, afterAdd)
	}
	afterDelete, err := patchDomainConfigText(afterAdd, "Finland", add, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("domain config did not round-trip:\n--- want\n%s\n--- got\n%s", before, afterDelete)
	}
}

func TestPatchCIDRConfigTextPreservesUntouchedBlocks(t *testing.T) {
	before := strings.Join([]string{
		"## Finland",
		"/Finland",
		"1.1.1.1/32",
		"",
		"## Russia",
		"/Russia",
		"10.0.0.0/8",
		"",
	}, "\n")
	req := openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "8.8.8.8/32"}
	afterAdd, err := patchCIDRConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	wantAdd := strings.Replace(before, "1.1.1.1/32\n", "1.1.1.1/32\n8.8.8.8/32\n", 1)
	if afterAdd != wantAdd {
		t.Fatalf("unexpected add diff:\n--- want\n%s\n--- got\n%s", wantAdd, afterAdd)
	}
	afterDelete, err := patchCIDRConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("CIDR config did not round-trip:\n--- want\n%s\n--- got\n%s", before, afterDelete)
	}
}

func TestPatchCIDRConfigTextRemovesNewEmptyBlock(t *testing.T) {
	before := strings.Join([]string{
		"## Existing disabled example",
		"#/Russia",
		"10.0.0.0/8",
		"",
	}, "\n")
	req := openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "203.0.113.42/32"}
	afterAdd, err := patchCIDRConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := patchCIDRConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("CIDR config did not round-trip after creating target:\n--- want\n%s\n--- got\n%s", before, afterDelete)
	}
}

func TestPatchCIDRConfigTextRoundTripsEmptyFile(t *testing.T) {
	req := openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "203.0.113.42/32"}
	afterAdd, err := patchCIDRConfigText("", "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := patchCIDRConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != "" {
		t.Fatalf("empty CIDR config did not round-trip: %q", afterDelete)
	}
}

func TestPatchCIDRConfigTextPreservesTrailingBlankLineForNewTarget(t *testing.T) {
	before := "/Russia\n10.0.0.0/8\n\n"
	req := openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "203.0.113.42/32"}
	afterAdd, err := patchCIDRConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := patchCIDRConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("CIDR config trailing blank line changed:\n--- want\n%q\n--- got\n%q", before, afterDelete)
	}
}

func TestPatchCIDRConfigTextPreservesMissingTrailingNewline(t *testing.T) {
	before := "##CIDR: geosite: TELEGRAM / geoip: TELEGRAM\n/Finland\ngeoip:telegram"
	req := openapi.CIDRRulePatchRequest{Kind: openapi.CIDRRulePatchRequestKindCidr, Value: "203.0.113.42/32"}
	afterAdd, err := patchCIDRConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := patchCIDRConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("CIDR config trailing newline changed:\n--- want\n%q\n--- got\n%q", before, afterDelete)
	}
}

func TestPatchDomainConfigTextPreservesMissingTrailingNewline(t *testing.T) {
	before := "example.com/Finland"
	req := openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "openai.com"}
	afterAdd, err := patchDomainConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := patchDomainConfigText(afterAdd, "Finland", req, false)
	if err != nil {
		t.Fatal(err)
	}
	if afterDelete != before {
		t.Fatalf("domain config trailing newline changed:\n--- want\n%q\n--- got\n%q", before, afterDelete)
	}
}

func TestPatchDomainConfigTextAllowsExistingDirectiveLikeComment(t *testing.T) {
	before := strings.Join([]string{
		"## Existing note",
		"##geosite: TELEGRAM / geoip: TELEGRAM",
		"example.com/Finland",
		"",
	}, "\n")
	req := openapi.DomainRulePatchRequest{Kind: openapi.DomainRulePatchRequestKindDomain, Value: "openai.com"}
	after, err := patchDomainConfigText(before, "Finland", req, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after, "##geosite: TELEGRAM / geoip: TELEGRAM\n") {
		t.Fatalf("existing comment changed:\n%s", after)
	}
}
