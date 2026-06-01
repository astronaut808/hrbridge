package agent

import (
	"net/http"
	"os"
	"os/exec"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.doctorReport())
}

func (s *Server) doctorReport() openapi.DoctorReport {
	var checks []openapi.DoctorCheck
	add := func(sev openapi.DoctorCheckSeverity, component, code, message, detail string) {
		check := openapi.DoctorCheck{Severity: sev, Component: component, Code: code, Message: message}
		if detail != "" {
			check.Detail = &detail
		}
		checks = append(checks, check)
	}
	for _, item := range []struct {
		component string
		path      string
	}{
		{"hrneo", s.cfg.HRNeoConf},
		{"domains", s.cfg.DomainConf},
		{"cidr", s.cfg.CIDRList},
		{"backup", s.cfg.BackupDir},
		{"audit", s.cfg.AuditLog},
		{"init", s.cfg.HRNeoInit},
	} {
		if _, err := os.Stat(item.path); err == nil {
			add(openapi.DoctorCheckSeverityOk, item.component, "path-ok", item.path+" exists", "")
		} else {
			add(openapi.DoctorCheckSeverityWarning, item.component, "path-missing", item.path+" is not available", err.Error())
		}
	}
	for _, cmd := range []struct {
		name       string
		args       []string
		lookupOnly bool
	}{
		{"hrneo", []string{"--version"}, false},
		{"ipset", []string{"--version"}, false},
		{"iptables", []string{"--version"}, false},
		{"ip6tables", []string{"--version"}, false},
		{"iptables-restore", nil, true},
		{"ip6tables-restore", nil, true},
		{"ip", []string{"-V"}, false},
	} {
		if cmd.lookupOnly {
			if path, err := exec.LookPath(cmd.name); err == nil {
				add(openapi.DoctorCheckSeverityOk, "command", "command-ok", cmd.name+" is executable", path)
			} else {
				add(openapi.DoctorCheckSeverityWarning, "command", "command-unavailable", cmd.name+" is not available", err.Error())
			}
			continue
		}
		if out, err := runCommand(cmd.name, cmd.args...); err == nil {
			add(openapi.DoctorCheckSeverityOk, "command", "command-ok", cmd.name+" is executable", out)
		} else {
			add(openapi.DoctorCheckSeverityWarning, "command", "command-unavailable", cmd.name+" is not available", commandErrorMessage(out, err))
		}
	}
	compat := s.compatibilityReport()
	for _, check := range compat.Checks {
		if check.Severity == openapi.CompatibilityCheckSeverityOk {
			continue
		}
		sev := openapi.DoctorCheckSeverityWarning
		if check.Severity == openapi.CompatibilityCheckSeverityError {
			sev = openapi.DoctorCheckSeverityError
		}
		add(sev, string(check.Component), check.Code, check.Message, stringPtrValue(check.Path))
	}
	ok := true
	for _, check := range checks {
		if check.Severity == openapi.DoctorCheckSeverityError {
			ok = false
			break
		}
	}
	return openapi.DoctorReport{Ok: ok, Checks: checks}
}

func stringPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
