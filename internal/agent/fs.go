package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	// codeql[go/path-injection] Callers pass fixed root-owned config paths,
	// fixed backup allowlist paths, or GeoData paths validated under geofile/.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	// codeql[go/path-injection] The temporary path is derived from the
	// validated destination path and never from a separate user-controlled name.
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) // #nosec G304 -- callers use configured files or validated geodata paths
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		// codeql[go/path-injection] tmp is derived from the already validated destination path.
		_ = os.Remove(tmp)
		return writeErr
	}
	if syncErr != nil {
		// codeql[go/path-injection] tmp is derived from the already validated destination path.
		_ = os.Remove(tmp)
		return syncErr
	}
	if closeErr != nil {
		// codeql[go/path-injection] tmp is derived from the already validated destination path.
		_ = os.Remove(tmp)
		return closeErr
	}
	// codeql[go/path-injection] Both paths are constrained to the same
	// validated destination selected by the caller.
	if err := os.Rename(tmp, path); err != nil {
		// codeql[go/path-injection] tmp is derived from the already validated destination path.
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
