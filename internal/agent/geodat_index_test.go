package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanGeoDataTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.dat")
	if err := os.WriteFile(path, geoDataDAT("RU", "US", "PRIVATE"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := scanGeoDataTags(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PRIVATE", "RU", "US"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected tags: want=%#v got=%#v", want, got)
	}
}

func TestScanGeoDataTagsRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.dat")
	if err := os.WriteFile(path, []byte("not-a-dat"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := scanGeoDataTags(path); err == nil {
		t.Fatal("expected malformed file error")
	}
}

func geoDataDAT(tags ...string) []byte {
	var out bytes.Buffer
	for _, tag := range tags {
		body := append([]byte{0x0a, byte(len(tag))}, []byte(tag)...)
		out.WriteByte(0x0a)
		out.WriteByte(byte(len(body)))
		out.Write(body)
	}
	return out.Bytes()
}
