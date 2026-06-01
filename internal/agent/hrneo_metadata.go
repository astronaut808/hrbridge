package agent

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

func (s *Server) handleHRNeoParamMetadata(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openapi.HRNeoParamMetadataResponse{
		SupportedHrneoVersion: supportedHRNeoVersion,
		Params:                hrneoParamMetadata(),
	})
}

func (s *Server) handleGetHRNeoDefault(w http.ResponseWriter, r *http.Request) {
	cfg := defaultHRNeoConfig()
	writeJSON(w, http.StatusOK, openapi.HRNeoDefaultConfigResponse{
		Config:         cfg,
		Content:        renderHRNeoConfig(cfg),
		RequiredAction: openapi.HRNeoDefaultConfigResponseRequiredActionRestart,
	})
}

func (s *Server) handleGenerateHRNeoDefault(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.GenerateDefaultHRNeoConfigRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		s.writeAudit(r, "config.generate-default", "hrneo", false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !checkRevisionPrecondition(r, s.cfg.HRNeoConf) {
		writeError(w, http.StatusPreconditionFailed, "config revision mismatch")
		return
	}

	backup, err := s.createBackup(configBackupReason("hrneo-generate-default"))
	if err != nil {
		s.writeAudit(r, "config.generate-default", "hrneo", false, "", err)
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	if err := atomicWriteFile(s.cfg.HRNeoConf, []byte(renderHRNeoConfig(defaultHRNeoConfig())), 0o600); err != nil {
		s.writeAudit(r, "config.generate-default", "hrneo", false, backup.Id, err)
		writeError(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}
	setRevisionHeader(w, fileRevision(s.cfg.HRNeoConf))

	resp := openapi.PutConfigResponse{
		Saved:          true,
		Backup:         backup,
		RequiredAction: openapi.PutConfigResponseRequiredActionRestart,
	}
	if req.Apply != nil && *req.Apply {
		out, err := s.runService("restart")
		resp.ApplyOutput = &out
		if err != nil {
			s.writeAudit(r, "config.generate-default", "hrneo", false, backup.Id, err)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		resp.Applied = true
	}
	s.writeAudit(r, "config.generate-default", "hrneo", true, backup.Id, nil)
	writeJSON(w, http.StatusOK, resp)
}

func hrneoParamMetadata() []openapi.HRNeoParamMetadata {
	restart := openapi.HRNeoParamMetadataRequiredActionRestart
	param := func(name, cliFlag string, valueType openapi.HRNeoParamMetadataValueType, defaultValue, description string, repeatable bool) openapi.HRNeoParamMetadata {
		return openapi.HRNeoParamMetadata{
			Name:           name,
			CliFlag:        cliFlag,
			ValueType:      valueType,
			DefaultValue:   defaultValue,
			Description:    description,
			Repeatable:     repeatable,
			RequiredAction: restart,
		}
	}
	return []openapi.HRNeoParamMetadata{
		param("autoStart", "--autoStart", openapi.HRNeoParamMetadataValueTypeBool, "true", "Allow daemon startup", false),
		param("watchlistPath", "--watchlistPath", openapi.HRNeoParamMetadataValueTypePath, "/opt/etc/HydraRoute/domain.conf", "Path to domain watchlist file", false),
		param("clearIPSet", "--clearIPSet", openapi.HRNeoParamMetadataValueTypeBool, "true", "Flush ipsets on startup", false),
		param("CIDR", "--CIDR", openapi.HRNeoParamMetadataValueTypeBool, "true", "Enable loading static CIDR blocks", false),
		param("CIDRfile", "--CIDRfile", openapi.HRNeoParamMetadataValueTypePath, "/opt/etc/HydraRoute/ip.list", "Path to CIDR list file", false),
		param("IpsetEnableTimeout", "--IpsetEnableTimeout", openapi.HRNeoParamMetadataValueTypeBool, "true", "Enable ipset entry timeout", false),
		param("IpsetTimeout", "--IpsetTimeout", openapi.HRNeoParamMetadataValueTypeInt, "21600", "Entry timeout in seconds", false),
		param("log", "--log", openapi.HRNeoParamMetadataValueTypeString, "off", "Log output mode", false),
		param("logfile", "--logfile", openapi.HRNeoParamMetadataValueTypePath, "/opt/var/log/LOGhrneo.log", "Log file path", false),
		param("DirectRouteEnabled", "--DirectRouteEnabled", openapi.HRNeoParamMetadataValueTypeBool, "true", "Enable direct interface routing", false),
		param("InterfaceFwMarkStart", "--InterfaceFwMarkStart", openapi.HRNeoParamMetadataValueTypePositiveInt, "12289", "Starting fwmark value", false),
		param("InterfaceTableStart", "--InterfaceTableStart", openapi.HRNeoParamMetadataValueTypePositiveInt, "301", "Starting routing table ID", false),
		param("GlobalRouting", "--GlobalRouting", openapi.HRNeoParamMetadataValueTypeBool, "false", "Override router policies for all traffic", false),
		param("ConntrackFlush", "--ConntrackFlush", openapi.HRNeoParamMetadataValueTypeBool, "true", "Flush conntrack on new IP", false),
		param("IpsetMaxElem", "--IpsetMaxElem", openapi.HRNeoParamMetadataValueTypePositiveInt, "262144", "Max entries per ipset", false),
		param("GeoIPFile", "--GeoIPFile", openapi.HRNeoParamMetadataValueTypeRepeatPath, "", "GeoIP .dat file", true),
		param("GeoSiteFile", "--GeoSiteFile", openapi.HRNeoParamMetadataValueTypeRepeatPath, "", "GeoSite .dat file", true),
		param("PolicyOrder", "--PolicyOrder", openapi.HRNeoParamMetadataValueTypePolicyOrder, "", "Comma-separated policy priority order", false),
		param("l7CaptureEnabled", "--l7CaptureEnabled", openapi.HRNeoParamMetadataValueTypeBool, "true", "Enable L7 TLS and HTTP capture via NFQUEUE", false),
		param("l7QueueNum", "--l7QueueNum", openapi.HRNeoParamMetadataValueTypePositiveInt, "210", "NFQUEUE number for L7 capture", false),
		param("l7EnableTLS", "--l7EnableTLS", openapi.HRNeoParamMetadataValueTypeBool, "true", "Parse TLS ClientHello SNI on dport 443", false),
		param("l7EnableHTTP", "--l7EnableHTTP", openapi.HRNeoParamMetadataValueTypeBool, "true", "Parse HTTP Host on dport 80", false),
		param("l7WanInterface", "--l7WanInterface", openapi.HRNeoParamMetadataValueTypeString, "", "WAN interface for L7 firewall rules", false),
		param("l7ConnbytesMax", "--l7ConnbytesMax", openapi.HRNeoParamMetadataValueTypePositiveInt, "8", "Connbytes upper bound for L7 firewall rule", false),
		param("l7TcpReasmEnabled", "--l7TcpReasmEnabled", openapi.HRNeoParamMetadataValueTypeBool, "true", "Enable TCP reassembly for long ClientHello", false),
		param("l7TcpReasmMaxEntries", "--l7TcpReasmMaxEntries", openapi.HRNeoParamMetadataValueTypePositiveInt, "256", "Max concurrent reassembly entries", false),
		param("l7TcpReasmTtlSec", "--l7TcpReasmTtlSec", openapi.HRNeoParamMetadataValueTypePositiveInt, "5", "TTL of incomplete reassembly entries", false),
	}
}
