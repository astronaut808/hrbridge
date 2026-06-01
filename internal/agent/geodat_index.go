package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const maxGeoDataTagBytes = 64

func scanGeoDataTags(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- geodata paths come from root-owned HR Neo configuration
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReaderSize(f, 64*1024)
	seen := map[string]bool{}
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
		body := &io.LimitedReader{R: r, N: int64(bodyLen)}
		bodyReader := bufio.NewReader(body)
		tag, err := readGeoDataEntryTag(bodyReader)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid entry: %w", path, err)
		}
		if _, err := io.Copy(io.Discard, bodyReader); err != nil {
			return nil, err
		}
		if body.N != 0 {
			return nil, fmt.Errorf("%s: truncated entry", path)
		}
		if tag != "" {
			seen[tag] = true
		}
	}

	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, nil
}

func readGeoDataEntryTag(r *bufio.Reader) (string, error) {
	field, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	if field != 0x0a {
		return "", fmt.Errorf("expected code field, got protobuf tag 0x%02x", field)
	}
	length, err := readProtoVarint(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	if length >= maxGeoDataTagBytes {
		return "", fmt.Errorf("code exceeds %d bytes", maxGeoDataTagBytes-1)
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(r, value); err != nil {
		return "", err
	}
	return strings.ToUpper(string(value)), nil
}

func readProtoVarint(r io.ByteReader) (uint64, error) {
	var value uint64
	for shift := uint(0); shift < 64; shift += 7 {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("varint overflow")
}
