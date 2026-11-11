package core

import (
	"errors"

	"github.com/BurntSushi/toml"
)

// Config mirrors conf/config.toml.
type Config struct {
	Device DeviceConfig
	Broker BrokerConfig
	Agent  AgentConfig
	Av     AvConfig
	Popup  PopupConfig
	Clean  CleanConfig
	Proc   ProcConfig
	Audit  AuditConfig
	Wol    WolConfig
}

type DeviceConfig struct {
	SchoolCode   string `toml:"school_code"`
	SN           string `toml:"sn"`
	DeviceSecret string `toml:"device_secret"`
	MqttToken    string `toml:"mqtt_token"`
}

type BrokerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type AgentConfig struct {
	HeartbeatSeconds int    `toml:"heartbeat_seconds"`
	LogLevel         string `toml:"log_level"`
	IPCToken         string `toml:"ipc_token"`
	MediaUploadURL   string `toml:"media_upload_url"`
	MediaUploadToken string `toml:"media_upload_token"`
	MpvPath          string `toml:"mpv_path"`
}

// AvConfig is the optional [av] section (local antivirus engine ability).
// Pointers default to true when the key is absent.
type AvConfig struct {
	Enabled           *bool    `toml:"enabled"`
	DbPath            string   `toml:"db_path"`
	QuarantineDir     string   `toml:"quarantine_dir"`
	Threads           int      `toml:"threads"`
	MaxFileSizeMB     int      `toml:"max_file_size_mb"`
	Exclusions        []string `toml:"exclusions"`
	EnableHeuristics  *bool    `toml:"enable_heuristics"`
	DefaultOnDetect   string   `toml:"default_on_detect"`
	ScanIntervalHours int      `toml:"scan_interval_hours"`
	ScanMode          string   `toml:"scan_mode"`
}

// PopupConfig is the optional [popup] section (popup/ad blocking ability).
type PopupConfig struct {
	Enabled    *bool  `toml:"enabled"`
	RulesPath  string `toml:"rules_path"`
	IntervalMS int    `toml:"interval_ms"`
	Action     string `toml:"action"`
	MaxEvents  int    `toml:"max_events"`
}

// CleanConfig is the optional [clean] section (junk cleanup ability).
type CleanConfig struct {
	Enabled             *bool    `toml:"enabled"`
	MinAgeHours         int      `toml:"min_age_hours"` // 0/absent = 24h, -1 = no guard
	Exclusions          []string `toml:"exclusions"`
	EnableCategories    []string `toml:"enable_categories"`
	DisableCategories   []string `toml:"disable_categories"`
	ScanIntervalHours   int      `toml:"scan_interval_hours"`
	RunIntervalHours    int      `toml:"run_interval_hours"`
	ScheduledCategories []string `toml:"scheduled_categories"`
}

// ProcConfig is the optional [proc] section (process acceleration /
// per-process traffic ability).
type ProcConfig struct {
	Enabled        *bool `toml:"enabled"`
	PollIntervalMS int   `toml:"poll_interval_ms"` // 0 = default 5s
}

// AuditConfig is the optional [audit] section (web behavior audit ability).
type AuditConfig struct {
	Enabled    *bool `toml:"enabled"`
	MaxRecords int   `toml:"max_records"` // per-source result cap; 0 = 200
}

// WolConfig is the optional [wol] section (LAN wake/discovery ability).
type WolConfig struct {
	Enabled *bool `toml:"enabled"`
	DefPort int   `toml:"default_port"` // magic packet port; 0 = 9
}

// LoadConfig reads and validates conf/config.toml.
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.Device.SchoolCode == "" || cfg.Device.SN == "" {
		return nil, errors.New("device.school_code and device.sn are required")
	}
	if cfg.Device.DeviceSecret == "" || cfg.Device.MqttToken == "" {
		return nil, errors.New("device.device_secret and device.mqtt_token are required")
	}
	if cfg.Broker.Host == "" || cfg.Broker.Port == 0 {
		return nil, errors.New("broker.host and broker.port are required")
	}
	if cfg.Av.ScanIntervalHours < 0 || cfg.Av.ScanIntervalHours > 8760 {
		return nil, errors.New("av.scan_interval_hours must be between 0 and 8760")
	}
	if cfg.Av.ScanMode != "" && cfg.Av.ScanMode != "quick" && cfg.Av.ScanMode != "full" {
		return nil, errors.New("av.scan_mode must be quick or full")
	}
	if cfg.Clean.ScanIntervalHours < 0 || cfg.Clean.ScanIntervalHours > 8760 || cfg.Clean.RunIntervalHours < 0 || cfg.Clean.RunIntervalHours > 8760 {
		return nil, errors.New("clean schedule intervals must be between 0 and 8760 hours")
	}
	if cfg.Clean.RunIntervalHours > 0 && len(cfg.Clean.ScheduledCategories) == 0 {
		return nil, errors.New("clean.scheduled_categories is required when clean.run_interval_hours is enabled")
	}
	return &cfg, nil
}
