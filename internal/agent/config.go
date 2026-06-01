package agent

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Version           = "0.1.0"
	DefaultConfigPath = "/opt/etc/HydraRoute/hrbridge.conf"

	DefaultHRNeoConf  = "/opt/etc/HydraRoute/hrneo.conf"
	DefaultDomainConf = "/opt/etc/HydraRoute/domain.conf"
	DefaultCIDRList   = "/opt/etc/HydraRoute/ip.list"
	DefaultBackupDir  = "/opt/etc/HydraRoute/backups"
	DefaultLogFile    = "/opt/var/log/LOGhrneo.log"
	DefaultAuditLog   = "/opt/var/log/hrbridge-audit.log"
	DefaultHRNeoPID   = "/var/run/hrneo.pid"
	DefaultInitScript = "/opt/etc/init.d/S99hrneo"
	DefaultListen     = "0.0.0.0:2080"
	DefaultRCIURL     = "http://127.0.0.1:79"
)

type Config struct {
	ConfigPath   string `json:"-"`
	Listen       string `json:"listen"`
	AuthToken    string `json:"-"`
	AllowOrigins string `json:"allowOrigins"`
	EnableTLS    bool   `json:"enableTLS"`
	CertFile     string `json:"certFile"`
	KeyFile      string `json:"keyFile"`
	BackupDir    string `json:"backupDir"`
	LogFile      string `json:"logFile"`
	AuditLog     string `json:"auditLog"`
	HRNeoConf    string `json:"hrneoConf"`
	DomainConf   string `json:"domainConf"`
	CIDRList     string `json:"cidrList"`
	HRNeoPID     string `json:"hrneoPid"`
	HRNeoInit    string `json:"hrneoInit"`
	RCIURL       string `json:"rciURL"`
}

func DefaultConfig() Config {
	return Config{
		Listen:     DefaultListen,
		BackupDir:  DefaultBackupDir,
		LogFile:    DefaultLogFile,
		AuditLog:   DefaultAuditLog,
		HRNeoConf:  DefaultHRNeoConf,
		DomainConf: DefaultDomainConf,
		CIDRList:   DefaultCIDRList,
		HRNeoPID:   DefaultHRNeoPID,
		HRNeoInit:  DefaultInitScript,
		RCIURL:     DefaultRCIURL,
	}
}

func LoadOrCreateConfig(path string) (Config, bool, error) {
	cfg := DefaultConfig()
	cfg.ConfigPath = path
	if err := loadKV(path, &cfg); err != nil {
		if !os.IsNotExist(err) {
			return cfg, false, err
		}
	}

	createdToken := false
	if cfg.AuthToken == "" {
		token, err := generateToken()
		if err != nil {
			return cfg, false, err
		}
		cfg.AuthToken = token
		createdToken = true
		if err := writeConfig(path, cfg); err != nil {
			return cfg, false, err
		}
	}

	return cfg, createdToken, nil
}

func loadKV(path string, cfg *Config) error {
	f, err := os.Open(path) // #nosec G304 -- config path is a local root-controlled startup argument
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		applyConfigKV(cfg, strings.TrimSpace(k), strings.TrimSpace(v))
	}
	return sc.Err()
}

func applyConfigKV(cfg *Config, key, val string) {
	switch key {
	case "listen":
		if val != "" {
			cfg.Listen = val
		}
	case "authToken":
		cfg.AuthToken = val
	case "allowOrigins":
		cfg.AllowOrigins = val
	case "enableTLS":
		cfg.EnableTLS = val == "true"
	case "certFile":
		cfg.CertFile = val
	case "keyFile":
		cfg.KeyFile = val
	case "backupDir":
		if val != "" {
			cfg.BackupDir = val
		}
	case "logFile":
		cfg.LogFile = val
	case "auditLog":
		cfg.AuditLog = val
	case "hrneoConf":
		if val != "" {
			cfg.HRNeoConf = val
		}
	case "domainConf":
		if val != "" {
			cfg.DomainConf = val
		}
	case "cidrList":
		if val != "" {
			cfg.CIDRList = val
		}
	case "hrneoPid":
		if val != "" {
			cfg.HRNeoPID = val
		}
	case "hrneoInit":
		if val != "" {
			cfg.HRNeoInit = val
		}
	case "rciURL":
		if val != "" {
			cfg.RCIURL = val
		}
	}
}

func writeConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body := fmt.Sprintf(`listen=%s
authToken=%s
allowOrigins=%s
enableTLS=%t
certFile=%s
keyFile=%s
backupDir=%s
logFile=%s
auditLog=%s
hrneoConf=%s
domainConf=%s
cidrList=%s
hrneoPid=%s
hrneoInit=%s
rciURL=%s
`, cfg.Listen, cfg.AuthToken, cfg.AllowOrigins, cfg.EnableTLS, cfg.CertFile,
		cfg.KeyFile, cfg.BackupDir, cfg.LogFile, cfg.AuditLog, cfg.HRNeoConf, cfg.DomainConf,
		cfg.CIDRList, cfg.HRNeoPID, cfg.HRNeoInit, cfg.RCIURL)
	return atomicWriteFile(path, []byte(body), 0o600)
}

func generateToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
