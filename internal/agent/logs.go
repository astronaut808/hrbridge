package agent

import (
	"bufio"
	"os"
)

func tailFile(path string, limit int) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- log path comes from root-owned HydraBridge configuration
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if limit <= 0 {
		limit = 300
	}
	ring := make([]string, limit)
	count := 0
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		ring[count%limit] = sc.Text()
		count++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	n := count
	if n > limit {
		n = limit
	}
	out := make([]string, 0, n)
	start := count - n
	for i := 0; i < n; i++ {
		out = append(out, ring[(start+i)%limit])
	}
	return out, nil
}
