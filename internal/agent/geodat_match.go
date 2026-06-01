package agent

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
)

func geoIPPrefixesContaining(paths []string, tag string, addr netip.Addr) ([]netip.Prefix, error) {
	var out []netip.Prefix
	seen := map[netip.Prefix]bool{}
	for _, path := range paths {
		prefixes, err := scanGeoIPPrefixesContaining(path, tag, addr)
		if err != nil {
			return nil, err
		}
		for _, prefix := range prefixes {
			if !seen[prefix] {
				seen[prefix] = true
				out = append(out, prefix)
			}
		}
	}
	return out, nil
}

func scanGeoIPPrefixesContaining(path, wantedTag string, addr netip.Addr) ([]netip.Prefix, error) {
	f, err := os.Open(path) // #nosec G304 -- geodata paths come from root-owned HR Neo configuration
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReaderSize(f, 64*1024)
	var out []netip.Prefix
	for {
		topTag, err := r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if topTag != 0x0a {
			return nil, fmt.Errorf("%s: unexpected top-level protobuf tag 0x%02x", path, topTag)
		}
		bodyLen, err := readProtoVarint(r)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid entry length: %w", path, err)
		}
		if bodyLen > maxGeoDataBytes {
			return nil, fmt.Errorf("%s: entry exceeds %d bytes", path, maxGeoDataBytes)
		}
		body := make([]byte, int(bodyLen))
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("%s: truncated entry: %w", path, err)
		}
		tag, rest, err := geoDataEntryCode(body)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid entry: %w", path, err)
		}
		if !strings.EqualFold(tag, wantedTag) {
			continue
		}
		prefixes, err := geoIPBodyPrefixesContaining(rest, addr)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid geoip:%s entry: %w", path, wantedTag, err)
		}
		out = append(out, prefixes...)
	}
	return out, nil
}

func geoDataEntryCode(body []byte) (string, []byte, error) {
	if len(body) == 0 || body[0] != 0x0a {
		return "", nil, fmt.Errorf("missing code field")
	}
	length, n, err := readProtoVarintBytes(body[1:])
	if err != nil {
		return "", nil, err
	}
	start := 1 + n
	lengthInt, err := checkedProtoLength(length, len(body)-start)
	if err != nil {
		return "", nil, err
	}
	end := start + lengthInt
	return strings.ToUpper(string(body[start:end])), body[end:], nil
}

func geoIPBodyPrefixesContaining(body []byte, addr netip.Addr) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for len(body) > 0 {
		field := body[0]
		body = body[1:]
		if field != 0x12 {
			var err error
			body, err = skipProtoField(field, body)
			if err != nil {
				return nil, err
			}
			continue
		}
		length, n, err := readProtoVarintBytes(body)
		if err != nil {
			return nil, err
		}
		body = body[n:]
		lengthInt, err := checkedProtoLength(length, len(body))
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
		prefix, ok, err := parseGeoIPCIDR(body[:lengthInt])
		if err != nil {
			return nil, err
		}
		if ok && prefix.Contains(addr) {
			out = append(out, prefix)
		}
		body = body[lengthInt:]
	}
	return out, nil
}

func parseGeoIPCIDR(body []byte) (netip.Prefix, bool, error) {
	var rawIP []byte
	var bits uint64
	for len(body) > 0 {
		field := body[0]
		body = body[1:]
		switch field {
		case 0x0a:
			length, n, err := readProtoVarintBytes(body)
			if err != nil {
				return netip.Prefix{}, false, err
			}
			body = body[n:]
			lengthInt, err := checkedProtoLength(length, len(body))
			if err != nil {
				return netip.Prefix{}, false, io.ErrUnexpectedEOF
			}
			rawIP = append([]byte(nil), body[:lengthInt]...)
			body = body[lengthInt:]
		case 0x10:
			var n int
			var err error
			bits, n, err = readProtoVarintBytes(body)
			if err != nil {
				return netip.Prefix{}, false, err
			}
			body = body[n:]
		default:
			var err error
			body, err = skipProtoField(field, body)
			if err != nil {
				return netip.Prefix{}, false, err
			}
		}
	}

	var addr netip.Addr
	switch len(rawIP) {
	case 4:
		addr = netip.AddrFrom4([4]byte(rawIP))
	case 16:
		addr = netip.AddrFrom16([16]byte(rawIP))
	default:
		return netip.Prefix{}, false, nil
	}
	if bits > 128 {
		return netip.Prefix{}, false, nil
	}
	bitsInt := int(bits) // #nosec G115 -- bounded to 128 above
	if bitsInt > addr.BitLen() {
		return netip.Prefix{}, false, nil
	}
	return netip.PrefixFrom(addr, bitsInt).Masked(), true, nil
}

func skipProtoField(field byte, body []byte) ([]byte, error) {
	switch field & 0x07 {
	case 0:
		_, n, err := readProtoVarintBytes(body)
		if err != nil {
			return nil, err
		}
		return body[n:], nil
	case 2:
		length, n, err := readProtoVarintBytes(body)
		if err != nil {
			return nil, err
		}
		lengthInt, err := checkedProtoLength(length, len(body)-n)
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
		return body[n+lengthInt:], nil
	default:
		return nil, fmt.Errorf("unsupported protobuf wire type %d", field&0x07)
	}
}

func checkedProtoLength(length uint64, available int) (int, error) {
	if available < 0 || length > uint64(available) { // #nosec G115 -- negative values are rejected before conversion
		return 0, io.ErrUnexpectedEOF
	}
	return int(length), nil // #nosec G115 -- bounded by an existing slice length
}

func readProtoVarintBytes(body []byte) (uint64, int, error) {
	var value uint64
	for i, b := range body {
		if i >= 10 {
			return 0, 0, fmt.Errorf("varint overflow")
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, io.ErrUnexpectedEOF
}
