package agent

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanGeoIPPrefixesContaining(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.dat")
	data := geoIPDataDAT("RU", "1.1.1.0/24", "2001:db8::/32")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := scanGeoIPPrefixesContaining(path, "ru", netip.MustParseAddr("1.1.1.42"))
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("1.1.1.0/24")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected prefixes: want=%#v got=%#v", want, got)
	}

	got, err = scanGeoIPPrefixesContaining(path, "RU", netip.MustParseAddr("2001:db8::42"))
	if err != nil {
		t.Fatal(err)
	}
	want = []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected IPv6 prefixes: want=%#v got=%#v", want, got)
	}
}

func geoIPDataDAT(tag string, prefixes ...string) []byte {
	var body bytes.Buffer
	body.WriteByte(0x0a)
	writeProtoBytes(&body, []byte(tag))
	for _, raw := range prefixes {
		prefix := netip.MustParsePrefix(raw)
		addr := prefix.Addr()
		ip := addr.AsSlice()
		var cidr bytes.Buffer
		cidr.WriteByte(0x0a)
		writeProtoBytes(&cidr, ip)
		cidr.WriteByte(0x10)
		writeProtoVarint(&cidr, uint64(prefix.Bits()))
		body.WriteByte(0x12)
		writeProtoBytes(&body, cidr.Bytes())
	}

	var out bytes.Buffer
	out.WriteByte(0x0a)
	writeProtoBytes(&out, body.Bytes())
	return out.Bytes()
}

func writeProtoBytes(b *bytes.Buffer, value []byte) {
	writeProtoVarint(b, uint64(len(value)))
	b.Write(value)
}

func writeProtoVarint(b *bytes.Buffer, value uint64) {
	for value >= 0x80 {
		b.WriteByte(byte(value) | 0x80)
		value >>= 7
	}
	b.WriteByte(byte(value))
}
