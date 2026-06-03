package agent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

const maxGeoDataBytes = 64 << 20

var geoDataHTTPClient = newGeoDataHTTPClient()

func (s *Server) handleGeoDataFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.geoDataFiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.GeoDataFilesResponse{Files: files})
}

func (s *Server) handleGeoDataUpload(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.GeoDataUploadRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxGeoDataBytes*2)).Decode(&req); err != nil {
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path, err := s.validatedGeoDataPath(req.Kind, req.Path)
	if err != nil {
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(data) > maxGeoDataBytes {
		err := fmt.Errorf("geodata file exceeds %d bytes", maxGeoDataBytes)
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.saveGeoData(req.Kind, path, data, addToConfigDefault(req.AddToConfig))
	if err != nil {
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	info, err := s.geoDataFileInfo(req.Kind, path, true)
	if err != nil {
		s.writeAudit(r, "geodata.upload", string(req.Kind), false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r, "geodata.upload", string(req.Kind), true, "", nil)
	writeJSON(w, http.StatusOK, openapi.GeoDataFileMutationResponse{
		Saved:         true,
		File:          info,
		ConfigUpdated: &updated,
	})
}

func (s *Server) handleGeoDataDownload(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var req openapi.GeoDataDownloadRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path, err := s.validatedGeoDataPath(req.Kind, req.Path)
	if err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	validatedURL, err := validateGeoDataURL(req.Url)
	if err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := downloadGeoData(r.Context(), validatedURL)
	if err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Sha256 != nil && !strings.EqualFold(strings.TrimSpace(*req.Sha256), sha256Hex(data)) {
		err := fmt.Errorf("sha256 mismatch")
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.saveGeoData(req.Kind, path, data, addToConfigDefault(req.AddToConfig))
	if err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	info, err := s.geoDataFileInfo(req.Kind, path, true)
	if err != nil {
		s.writeAudit(r, "geodata.download", string(req.Kind), false, "", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeAudit(r, "geodata.download", string(req.Kind), true, "", nil)
	writeJSON(w, http.StatusOK, openapi.GeoDataFileMutationResponse{
		Saved:         true,
		File:          info,
		ConfigUpdated: &updated,
	})
}

func (s *Server) handleGeoDataReferences(w http.ResponseWriter, r *http.Request) {
	refs, err := s.geoDataReferences()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

func (s *Server) handleGeoDataTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.geoDataTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleGeoDataValidate(w http.ResponseWriter, r *http.Request) {
	issues, err := s.validateGeoDataConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, validationResponse(issues))
}

func (s *Server) geoDataTags() (openapi.GeoDataTagsResponse, error) {
	cfg, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.GeoDataTagsResponse{}, err
	}
	geoip := map[string]bool{}
	geosite := map[string]bool{}
	for _, path := range stringSliceValue(cfg.GeoIPFile) {
		tags, err := scanGeoDataTags(path)
		if err != nil {
			return openapi.GeoDataTagsResponse{}, err
		}
		for _, tag := range tags {
			geoip[strings.ToUpper(tag)] = true
		}
	}
	for _, path := range stringSliceValue(cfg.GeoSiteFile) {
		tags, err := scanGeoDataTags(path)
		if err != nil {
			return openapi.GeoDataTagsResponse{}, err
		}
		for _, tag := range tags {
			geosite[strings.ToLower(tag)] = true
		}
	}
	return openapi.GeoDataTagsResponse{Geoip: sortedSet(geoip), Geosite: sortedSet(geosite)}, nil
}

func (s *Server) geoDataFiles() ([]openapi.GeoDataFileInfo, error) {
	cfg, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var out []openapi.GeoDataFileInfo
	for _, path := range stringSliceValue(cfg.GeoIPFile) {
		info, err := s.geoDataFileInfo(openapi.GeoDataKindGeoip, path, true)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	for _, path := range stringSliceValue(cfg.GeoSiteFile) {
		info, err := s.geoDataFileInfo(openapi.GeoDataKindGeosite, path, true)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *Server) geoDataReferences() (openapi.GeoDataReferencesResponse, error) {
	domains, err := parseDomainConfigFile(s.cfg.DomainConf)
	if err != nil && !os.IsNotExist(err) {
		return openapi.GeoDataReferencesResponse{}, err
	}
	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil && !os.IsNotExist(err) {
		return openapi.GeoDataReferencesResponse{}, err
	}
	geoipSet := map[string]bool{}
	geositeSet := map[string]bool{}
	for _, block := range cidr.Blocks {
		if !block.Enabled {
			continue
		}
		for _, tag := range block.Geoip {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				geoipSet[tag] = true
			}
		}
	}
	for _, target := range domains.Targets {
		if !target.Enabled {
			continue
		}
		for _, tag := range target.Geosite {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				geositeSet[tag] = true
			}
		}
	}
	return openapi.GeoDataReferencesResponse{
		Geoip:   sortedSet(geoipSet),
		Geosite: sortedSet(geositeSet),
	}, nil
}

func (s *Server) validateGeoDataConfig() ([]openapi.ValidationIssue, error) {
	var issues []openapi.ValidationIssue
	cfg, _, err := parseHRNeoConfigFile(s.cfg.HRNeoConf)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	refs, err := s.geoDataReferences()
	if err != nil {
		return nil, err
	}
	if len(refs.Geoip) > 0 && len(stringSliceValue(cfg.GeoIPFile)) == 0 {
		issues = append(issues, validationIssue(openapi.Error,
			"geoip-file-missing", "ip.list references geoip tags but GeoIPFile is not configured", "", "GeoIPFile"))
	}
	if len(refs.Geosite) > 0 && len(stringSliceValue(cfg.GeoSiteFile)) == 0 {
		issues = append(issues, validationIssue(openapi.Error,
			"geosite-file-missing", "domain.conf references geosite tags but GeoSiteFile is not configured", "", "GeoSiteFile"))
	}
	for _, path := range stringSliceValue(cfg.GeoIPFile) {
		appendGeoDataFileIssue(&issues, "geoip-file-not-found", path)
	}
	for _, path := range stringSliceValue(cfg.GeoSiteFile) {
		appendGeoDataFileIssue(&issues, "geosite-file-not-found", path)
	}
	geoipTags, geoipIndexed := appendGeoDataIndexIssues(&issues, "geoip", stringSliceValue(cfg.GeoIPFile))
	geositeTags, geositeIndexed := appendGeoDataIndexIssues(&issues, "geosite", stringSliceValue(cfg.GeoSiteFile))
	if geoipIndexed {
		for _, tag := range refs.Geoip {
			if !geoipTags[strings.ToUpper(tag)] {
				issues = append(issues, validationIssue(openapi.Warning,
					"geoip-tag-not-found", "geoip:"+tag+" is not present in configured .dat files", "", "geoip:"+tag))
			}
		}
	}
	if geositeIndexed {
		for _, tag := range refs.Geosite {
			if !geositeTags[strings.ToUpper(tag)] {
				issues = append(issues, validationIssue(openapi.Warning,
					"geosite-tag-not-found", "geosite:"+tag+" is not present in configured .dat files", "", "geosite:"+tag))
			}
		}
	}
	cidr, err := parseCIDRConfigFile(s.cfg.CIDRList)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, block := range cidr.Blocks {
		if block.Enabled || !strings.EqualFold(strings.TrimSpace(block.Target), "Too-big-geoip-tag") {
			continue
		}
		for _, tag := range block.Geoip {
			issues = append(issues, validationIssue(openapi.Warning,
				"geoip-tag-auto-disabled", "geoip:"+tag+" was disabled by HR Neo because it exceeds IpsetMaxElem", block.Target, "geoip:"+tag))
		}
	}
	return issues, nil
}

func appendGeoDataIndexIssues(issues *[]openapi.ValidationIssue, kind string, paths []string) (map[string]bool, bool) {
	tags := map[string]bool{}
	indexed := false
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		found, err := scanGeoDataTags(path)
		if err != nil {
			if !os.IsNotExist(err) {
				*issues = append(*issues, validationIssue(openapi.Warning,
					kind+"-file-invalid", err.Error(), "", path))
			}
			continue
		}
		indexed = true
		for _, tag := range found {
			tags[strings.ToUpper(tag)] = true
		}
	}
	return tags, indexed
}

func appendGeoDataFileIssue(issues *[]openapi.ValidationIssue, code, path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		severity := openapi.Warning
		if os.IsNotExist(err) {
			*issues = append(*issues, validationIssue(severity, code, path+" does not exist", "", path))
			return
		}
		*issues = append(*issues, validationIssue(severity, code, err.Error(), "", path))
	}
}

func sortedSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Server) saveGeoData(kind openapi.GeoDataKind, path string, data []byte, addToConfig bool) (bool, error) {
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return false, err
	}
	if !addToConfig {
		return false, nil
	}
	content, err := os.ReadFile(s.cfg.HRNeoConf)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		content = []byte(renderHRNeoConfig(defaultHRNeoConfig()))
	}
	key := ""
	switch kind {
	case openapi.GeoDataKindGeoip:
		key = "GeoIPFile"
	case openapi.GeoDataKindGeosite:
		key = "GeoSiteFile"
	default:
		return false, fmt.Errorf("invalid geodata kind")
	}
	patched, updated := appendHRNeoRepeatValue(string(content), key, path)
	if !updated {
		return false, nil
	}
	backup, err := s.createBackup(configBackupReason("geodata-config"))
	if err != nil {
		return false, err
	}
	_ = backup
	return true, atomicWriteFile(s.cfg.HRNeoConf, []byte(patched), 0o600)
}

func (s *Server) geoDataFileInfo(kind openapi.GeoDataKind, path string, configured bool) (openapi.GeoDataFileInfo, error) {
	info := openapi.GeoDataFileInfo{
		Kind:       kind,
		Path:       path,
		Configured: configured,
		Exists:     false,
	}
	path, err := s.validatedGeoDataPath(kind, path)
	if err != nil {
		return info, err
	}
	info.Path = path
	// codeql[go/path-injection] GeoData paths are constrained by
	// validatedGeoDataPath to geofile/ next to the root-owned HR Neo config.
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return info, err
	}
	info.Exists = true
	size := st.Size()
	mod := st.ModTime().UTC()
	info.Size = &size
	info.Modified = &mod
	// codeql[go/path-injection] GeoData paths are constrained by
	// validatedGeoDataPath to geofile/ next to the root-owned HR Neo config.
	data, err := os.ReadFile(path) // #nosec G304 -- configured geodata paths come from root-owned HR Neo configuration
	if err == nil {
		sum := sha256Hex(data)
		info.Sha256 = &sum
	}
	return info, nil
}

func (s *Server) validatedGeoDataPath(kind openapi.GeoDataKind, path string) (string, error) {
	if !kind.Valid() {
		return "", fmt.Errorf("kind must be one of: geoip, geosite")
	}
	if err := validatePathValue(path); err != nil {
		return "", err
	}
	base := filepath.Join(filepath.Dir(s.cfg.HRNeoConf), "geofile")
	base, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("geodata path must be inside %s", base)
	}
	return path, nil
}

type validatedGeoDataURL string

func downloadGeoData(ctx context.Context, rawURL validatedGeoDataURL) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(rawURL), nil)
	if err != nil {
		return nil, err
	}
	// codeql[go/request-forgery] The URL is accepted only after
	// validateGeoDataURL, redirects are revalidated, proxy use is disabled,
	// and the transport dials only public IP addresses.
	resp, err := geoDataHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGeoDataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxGeoDataBytes {
		return nil, fmt.Errorf("geodata file exceeds %d bytes", maxGeoDataBytes)
	}
	return data, nil
}

func newGeoDataHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialPublicAddress
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			_, err := validateGeoDataURL(req.URL.String())
			return err
		},
	}
}

func validateGeoDataURL(rawURL string) (validatedGeoDataURL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https")
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("url host must not be empty")
	}
	if u.User != nil {
		return "", fmt.Errorf("url userinfo is not allowed")
	}
	if addr, err := netip.ParseAddr(u.Hostname()); err == nil && !isPublicAddress(addr) {
		return "", fmt.Errorf("url host must resolve to a public address")
	}
	return validatedGeoDataURL(u.String()), nil
}

func dialPublicAddress(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{}
	for _, addr := range addrs {
		if !isPublicAddress(addr) {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("url host must resolve to a public address")
}

func isPublicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsValid() &&
		addr.IsGlobalUnicast() &&
		!addr.IsPrivate() &&
		!addr.IsLoopback() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsLinkLocalMulticast() &&
		!addr.IsUnspecified()
}

func addToConfigDefault(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
