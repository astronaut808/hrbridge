package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

type HRNeoStatus = openapi.HRNeoStatus

func (s *Server) handleService(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := s.runService(action)
		statusCode := http.StatusOK
		if err != nil {
			statusCode = http.StatusInternalServerError
		}
		s.writeAudit(r, "service."+action, "hrneo", err == nil, "", err)
		writeJSON(w, statusCode, openapi.ServiceActionResponse{
			Action: openapi.ServiceActionResponseAction(action),
			Ok:     err == nil,
			Output: out,
			Status: s.hrneoStatus(),
		})
	}
}

func (s *Server) runService(action string) (string, error) {
	switch action {
	case "reload":
		pid, err := readPID(s.cfg.HRNeoPID)
		if err != nil {
			return "", err
		}
		if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
			return "", err
		}
		return fmt.Sprintf("sent SIGUSR1 to %d", pid), nil
	case "start", "stop", "restart":
		return runCommand(s.cfg.HRNeoInit, action)
	default:
		return "", fmt.Errorf("unsupported service action: %s", action)
	}
}

func (s *Server) hrneoStatus() HRNeoStatus {
	st := HRNeoStatus{Installed: fileExists(s.cfg.HRNeoInit)}
	if pid, err := readPID(s.cfg.HRNeoPID); err == nil {
		if processExists(pid) {
			st.Running = true
			st.Pid = &pid
			uptime := processUptime(pid)
			st.UptimeSec = &uptime
		}
	}
	if out, err := runCommand("hrneo", "--version"); err == nil {
		version := strings.TrimSpace(out)
		st.Version = &version
	}
	return st
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) // #nosec G204 -- callers use fixed utilities or the root-owned HR Neo init path
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- PID path comes from root-owned HydraBridge configuration
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file: %s", path)
	}
	return pid, nil
}

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func processUptime(pid int) int64 {
	st, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return 0
	}
	return int64(time.Since(st.ModTime()).Seconds())
}

func sinceSeconds(t time.Time) int {
	return int(time.Since(t).Seconds())
}
