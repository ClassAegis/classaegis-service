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
	return &cfg, nil
}
