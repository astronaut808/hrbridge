package agent

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/astronaut808/hrbridge/internal/openapi"
)

var hrneoConfigOrder = []string{
	"autoStart",
	"watchlistPath",
	"clearIPSet",
	"CIDR",
	"CIDRfile",
	"IpsetEnableTimeout",
	"IpsetTimeout",
	"log",
	"logfile",
	"DirectRouteEnabled",
	"InterfaceFwMarkStart",
	"InterfaceTableStart",
	"GlobalRouting",
	"ConntrackFlush",
	"IpsetMaxElem",
	"GeoIPFile",
	"GeoSiteFile",
	"PolicyOrder",
	"l7CaptureEnabled",
	"l7QueueNum",
	"l7EnableTLS",
	"l7EnableHTTP",
	"l7WanInterface",
	"l7ConnbytesMax",
	"l7TcpReasmEnabled",
	"l7TcpReasmMaxEntries",
	"l7TcpReasmTtlSec",
}

func parseHRNeoConfigFile(path string) (openapi.HRNeoConfig, map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 -- HR Neo config path comes from root-owned HydraBridge configuration
	if err != nil {
		return openapi.HRNeoConfig{}, nil, err
	}
	defer func() { _ = f.Close() }()

	cfg := openapi.HRNeoConfig{}
	unknown := make(map[string]string)
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if !setHRNeoConfigValue(&cfg, key, val) {
			unknown[key] = val
		}
	}
	if err := sc.Err(); err != nil {
		return openapi.HRNeoConfig{}, nil, err
	}
	return cfg, unknown, nil
}

func setHRNeoConfigValue(cfg *openapi.HRNeoConfig, key, val string) bool {
	switch key {
	case "autoStart":
		cfg.AutoStart = boolPtr(parseBoolCompat(val))
	case "watchlistPath":
		cfg.WatchlistPath = strPtr(val)
	case "clearIPSet":
		cfg.ClearIPSet = boolPtr(parseBoolCompat(val))
	case "CIDR":
		cfg.CIDR = boolPtr(parseBoolCompat(val))
	case "CIDRfile":
		cfg.CIDRfile = strPtr(val)
	case "IpsetEnableTimeout":
		cfg.IpsetEnableTimeout = boolPtr(parseBoolCompat(val))
	case "IpsetTimeout":
		cfg.IpsetTimeout = intPtr(parseIntCompat(val))
	case "log":
		logVal := openapi.HRNeoConfigLog(val)
		cfg.Log = &logVal
	case "logfile":
		cfg.Logfile = strPtr(val)
	case "DirectRouteEnabled":
		cfg.DirectRouteEnabled = boolPtr(parseBoolCompat(val))
	case "InterfaceFwMarkStart":
		cfg.InterfaceFwMarkStart = intPtr(parseIntCompat(val))
	case "InterfaceTableStart":
		cfg.InterfaceTableStart = intPtr(parseIntCompat(val))
	case "GlobalRouting":
		cfg.GlobalRouting = boolPtr(parseBoolCompat(val))
	case "ConntrackFlush":
		cfg.ConntrackFlush = boolPtr(parseBoolCompat(val))
	case "IpsetMaxElem":
		cfg.IpsetMaxElem = intPtr(parseIntCompat(val))
	case "GeoIPFile":
		appendString(&cfg.GeoIPFile, val)
	case "GeoSiteFile":
		appendString(&cfg.GeoSiteFile, val)
	case "PolicyOrder":
		var values []string
		for _, item := range strings.Split(val, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				values = append(values, item)
			}
		}
		cfg.PolicyOrder = &values
	case "l7CaptureEnabled":
		cfg.L7CaptureEnabled = boolPtr(parseBoolCompat(val))
	case "l7QueueNum":
		cfg.L7QueueNum = intPtr(parseIntCompat(val))
	case "l7EnableTLS":
		cfg.L7EnableTLS = boolPtr(parseBoolCompat(val))
	case "l7EnableHTTP":
		cfg.L7EnableHTTP = boolPtr(parseBoolCompat(val))
	case "l7WanInterface":
		cfg.L7WanInterface = strPtr(val)
	case "l7ConnbytesMax":
		cfg.L7ConnbytesMax = intPtr(parseIntCompat(val))
	case "l7TcpReasmEnabled":
		cfg.L7TcpReasmEnabled = boolPtr(parseBoolCompat(val))
	case "l7TcpReasmMaxEntries":
		cfg.L7TcpReasmMaxEntries = intPtr(parseIntCompat(val))
	case "l7TcpReasmTtlSec":
		cfg.L7TcpReasmTtlSec = intPtr(parseIntCompat(val))
	default:
		return false
	}
	return true
}

func defaultHRNeoConfig() openapi.HRNeoConfig {
	logMode := openapi.Off
	emptyGeoIP := []string{}
	emptyGeoSite := []string{}
	emptyOrder := []string{}
	return openapi.HRNeoConfig{
		AutoStart:            boolPtr(true),
		WatchlistPath:        strPtr("/opt/etc/HydraRoute/domain.conf"),
		ClearIPSet:           boolPtr(true),
		CIDR:                 boolPtr(true),
		CIDRfile:             strPtr("/opt/etc/HydraRoute/ip.list"),
		IpsetEnableTimeout:   boolPtr(true),
		IpsetTimeout:         intPtr(21600),
		Log:                  &logMode,
		Logfile:              strPtr("/opt/var/log/LOGhrneo.log"),
		DirectRouteEnabled:   boolPtr(true),
		InterfaceFwMarkStart: intPtr(12289),
		InterfaceTableStart:  intPtr(301),
		GlobalRouting:        boolPtr(false),
		ConntrackFlush:       boolPtr(true),
		IpsetMaxElem:         intPtr(262144),
		GeoIPFile:            &emptyGeoIP,
		GeoSiteFile:          &emptyGeoSite,
		PolicyOrder:          &emptyOrder,
		L7CaptureEnabled:     boolPtr(true),
		L7QueueNum:           intPtr(210),
		L7EnableTLS:          boolPtr(true),
		L7EnableHTTP:         boolPtr(true),
		L7WanInterface:       strPtr(""),
		L7ConnbytesMax:       intPtr(8),
		L7TcpReasmEnabled:    boolPtr(true),
		L7TcpReasmMaxEntries: intPtr(256),
		L7TcpReasmTtlSec:     intPtr(5),
	}
}

func mergeHRNeoConfig(base, patch openapi.HRNeoConfig) openapi.HRNeoConfig {
	out := base
	if patch.AutoStart != nil {
		out.AutoStart = patch.AutoStart
	}
	if patch.WatchlistPath != nil {
		out.WatchlistPath = patch.WatchlistPath
	}
	if patch.ClearIPSet != nil {
		out.ClearIPSet = patch.ClearIPSet
	}
	if patch.CIDR != nil {
		out.CIDR = patch.CIDR
	}
	if patch.CIDRfile != nil {
		out.CIDRfile = patch.CIDRfile
	}
	if patch.IpsetEnableTimeout != nil {
		out.IpsetEnableTimeout = patch.IpsetEnableTimeout
	}
	if patch.IpsetTimeout != nil {
		out.IpsetTimeout = patch.IpsetTimeout
	}
	if patch.Log != nil {
		out.Log = patch.Log
	}
	if patch.Logfile != nil {
		out.Logfile = patch.Logfile
	}
	if patch.DirectRouteEnabled != nil {
		out.DirectRouteEnabled = patch.DirectRouteEnabled
	}
	if patch.InterfaceFwMarkStart != nil {
		out.InterfaceFwMarkStart = patch.InterfaceFwMarkStart
	}
	if patch.InterfaceTableStart != nil {
		out.InterfaceTableStart = patch.InterfaceTableStart
	}
	if patch.GlobalRouting != nil {
		out.GlobalRouting = patch.GlobalRouting
	}
	if patch.ConntrackFlush != nil {
		out.ConntrackFlush = patch.ConntrackFlush
	}
	if patch.IpsetMaxElem != nil {
		out.IpsetMaxElem = patch.IpsetMaxElem
	}
	if patch.GeoIPFile != nil {
		out.GeoIPFile = patch.GeoIPFile
	}
	if patch.GeoSiteFile != nil {
		out.GeoSiteFile = patch.GeoSiteFile
	}
	if patch.PolicyOrder != nil {
		out.PolicyOrder = patch.PolicyOrder
	}
	if patch.L7CaptureEnabled != nil {
		out.L7CaptureEnabled = patch.L7CaptureEnabled
	}
	if patch.L7QueueNum != nil {
		out.L7QueueNum = patch.L7QueueNum
	}
	if patch.L7EnableTLS != nil {
		out.L7EnableTLS = patch.L7EnableTLS
	}
	if patch.L7EnableHTTP != nil {
		out.L7EnableHTTP = patch.L7EnableHTTP
	}
	if patch.L7WanInterface != nil {
		out.L7WanInterface = patch.L7WanInterface
	}
	if patch.L7ConnbytesMax != nil {
		out.L7ConnbytesMax = patch.L7ConnbytesMax
	}
	if patch.L7TcpReasmEnabled != nil {
		out.L7TcpReasmEnabled = patch.L7TcpReasmEnabled
	}
	if patch.L7TcpReasmMaxEntries != nil {
		out.L7TcpReasmMaxEntries = patch.L7TcpReasmMaxEntries
	}
	if patch.L7TcpReasmTtlSec != nil {
		out.L7TcpReasmTtlSec = patch.L7TcpReasmTtlSec
	}
	return out
}

func renderHRNeoConfig(cfg openapi.HRNeoConfig) string {
	var b strings.Builder
	for _, key := range hrneoConfigOrder {
		writeHRNeoConfigKey(&b, key, cfg)
	}
	return b.String()
}

func renderHRNeoConfigPreservingUnknown(cfg openapi.HRNeoConfig, unknown map[string]string) string {
	out := renderHRNeoConfig(cfg)
	if len(unknown) == 0 {
		return out
	}
	var b strings.Builder
	b.WriteString(out)
	keys := make([]string, 0, len(unknown))
	for key := range unknown {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.TrimSpace(key) == "" || hasLineBreak(key) || hasLineBreak(unknown[key]) {
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", key, unknown[key])
	}
	return b.String()
}

func writeHRNeoConfigKey(b *strings.Builder, key string, cfg openapi.HRNeoConfig) {
	switch key {
	case "autoStart":
		writeBool(b, key, cfg.AutoStart)
	case "watchlistPath":
		writeString(b, key, cfg.WatchlistPath)
	case "clearIPSet":
		writeBool(b, key, cfg.ClearIPSet)
	case "CIDR":
		writeBool(b, key, cfg.CIDR)
	case "CIDRfile":
		writeString(b, key, cfg.CIDRfile)
	case "IpsetEnableTimeout":
		writeBool(b, key, cfg.IpsetEnableTimeout)
	case "IpsetTimeout":
		writeInt(b, key, cfg.IpsetTimeout)
	case "log":
		if cfg.Log != nil {
			fmt.Fprintf(b, "%s=%s\n", key, *cfg.Log)
		}
	case "logfile":
		writeString(b, key, cfg.Logfile)
	case "DirectRouteEnabled":
		writeBool(b, key, cfg.DirectRouteEnabled)
	case "InterfaceFwMarkStart":
		writeInt(b, key, cfg.InterfaceFwMarkStart)
	case "InterfaceTableStart":
		writeInt(b, key, cfg.InterfaceTableStart)
	case "GlobalRouting":
		writeBool(b, key, cfg.GlobalRouting)
	case "ConntrackFlush":
		writeBool(b, key, cfg.ConntrackFlush)
	case "IpsetMaxElem":
		writeInt(b, key, cfg.IpsetMaxElem)
	case "GeoIPFile":
		writeRepeatString(b, key, cfg.GeoIPFile)
	case "GeoSiteFile":
		writeRepeatString(b, key, cfg.GeoSiteFile)
	case "PolicyOrder":
		if cfg.PolicyOrder != nil {
			fmt.Fprintf(b, "%s=%s\n", key, strings.Join(*cfg.PolicyOrder, ","))
		}
	case "l7CaptureEnabled":
		writeBool(b, key, cfg.L7CaptureEnabled)
	case "l7QueueNum":
		writeInt(b, key, cfg.L7QueueNum)
	case "l7EnableTLS":
		writeBool(b, key, cfg.L7EnableTLS)
	case "l7EnableHTTP":
		writeBool(b, key, cfg.L7EnableHTTP)
	case "l7WanInterface":
		writeString(b, key, cfg.L7WanInterface)
	case "l7ConnbytesMax":
		writeInt(b, key, cfg.L7ConnbytesMax)
	case "l7TcpReasmEnabled":
		writeBool(b, key, cfg.L7TcpReasmEnabled)
	case "l7TcpReasmMaxEntries":
		writeInt(b, key, cfg.L7TcpReasmMaxEntries)
	case "l7TcpReasmTtlSec":
		writeInt(b, key, cfg.L7TcpReasmTtlSec)
	}
}

func writeBool(b *strings.Builder, key string, v *bool) {
	if v == nil {
		return
	}
	if *v {
		fmt.Fprintf(b, "%s=true\n", key)
	} else {
		fmt.Fprintf(b, "%s=false\n", key)
	}
}

func writeString(b *strings.Builder, key string, v *string) {
	if v != nil {
		fmt.Fprintf(b, "%s=%s\n", key, *v)
	}
}

func writeInt(b *strings.Builder, key string, v *int) {
	if v != nil {
		fmt.Fprintf(b, "%s=%d\n", key, *v)
	}
}

func writeRepeatString(b *strings.Builder, key string, values *[]string) {
	if values == nil {
		return
	}
	if len(*values) == 0 {
		fmt.Fprintf(b, "%s=\n", key)
		return
	}
	for _, value := range *values {
		fmt.Fprintf(b, "%s=%s\n", key, value)
	}
}

func parseBoolCompat(v string) bool {
	return v == "true"
}

func parseIntCompat(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func intPtr(v int) *int       { return &v }

func appendString(dst **[]string, val string) {
	if *dst == nil {
		values := []string{}
		*dst = &values
	}
	values := append(**dst, val)
	*dst = &values
}
