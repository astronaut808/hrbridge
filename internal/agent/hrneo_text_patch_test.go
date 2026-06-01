package agent

import "testing"

func TestReplaceHRNeoConfigValuePreservesUnrelatedText(t *testing.T) {
	input := "# keep this note\nlog=off\nPolicyOrder=Old\nFutureKey=value\nPolicyOrder=Duplicate\n"
	got := replaceHRNeoConfigValue(input, "PolicyOrder", "Finland,Russia")
	want := "# keep this note\nlog=off\nPolicyOrder=Finland,Russia\nFutureKey=value\n"
	if got != want {
		t.Fatalf("unexpected patch:\nwant=%q\n got=%q", want, got)
	}
}

func TestAppendHRNeoRepeatValuePreservesUnrelatedText(t *testing.T) {
	input := "# keep this note\nGeoIPFile=/opt/geoip.dat\nFutureKey=value\n"
	got, updated := appendHRNeoRepeatValue(input, "GeoIPFile", "/opt/geoip-extra.dat")
	want := "# keep this note\nGeoIPFile=/opt/geoip.dat\nFutureKey=value\nGeoIPFile=/opt/geoip-extra.dat\n"
	if !updated || got != want {
		t.Fatalf("unexpected patch: updated=%v\nwant=%q\n got=%q", updated, want, got)
	}
	got, updated = appendHRNeoRepeatValue(got, "GeoIPFile", " /opt/geoip-extra.dat ")
	if updated || got != want {
		t.Fatalf("duplicate append changed config: updated=%v got=%q", updated, got)
	}
}
