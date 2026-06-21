// Package config loads cylon's bootstrap configuration.
//
// Only server/bootstrap settings live here; runtime data (gateway, tags) lives
// in the database and is managed via the API/UI. Precedence is
// environment (CYLON_*) -> config file -> built-in default.
package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// Config is the full bootstrap configuration tree.
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Store   StoreConfig   `mapstructure:"store"`
	Gateway GatewayConfig `mapstructure:"gateway"`
	Sim     SimConfig     `mapstructure:"sim"`
}

// ServerConfig holds HTTP/metrics/logging settings.
type ServerConfig struct {
	HTTPListen    string `mapstructure:"http_listen"`
	MetricsListen string `mapstructure:"metrics_listen"`
	LogLevel      string `mapstructure:"log_level"`
}

// StoreConfig holds the SQLite database location.
type StoreConfig struct {
	Path string `mapstructure:"path"`
}

// GatewayConfig holds gateway bootstrap settings. The EUI is empty by default
// and generated+persisted on first run.
type GatewayConfig struct {
	EUI        string           `mapstructure:"eui"`
	EUIPrefix  string           `mapstructure:"eui_prefix"`
	TCPListen  string           `mapstructure:"tcp_listen"`
	Connection ConnectionConfig `mapstructure:"connection"`
}

// ConnectionConfig holds AWS connectivity settings (credential volume layout).
type ConnectionConfig struct {
	CredsDir string `mapstructure:"creds_dir"`
}

// SimConfig holds simulation behaviour knobs.
type SimConfig struct {
	Realtime bool `mapstructure:"realtime"`
}

var (
	validLogLevels = map[string]bool{
		"debug": true, "info": true, "warning": true, "warn": true, "error": true,
	}
	euiRe       = regexp.MustCompile(`^[0-9a-fA-F]{16}$`)
	euiPrefixRe = regexp.MustCompile(`^([0-9a-fA-F]{2}){1,8}$`)
)

// Default returns a Config populated with built-in defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPListen:    ":8080",
			MetricsListen: ":9100",
			LogLevel:      "info",
		},
		Store: StoreConfig{
			Path: "/var/lib/cylon/cylon.db",
		},
		Gateway: GatewayConfig{
			EUI:       "",
			TCPListen: ":6000",
			Connection: ConnectionConfig{
				CredsDir: "/etc/cylon/creds",
			},
		},
		Sim: SimConfig{
			Realtime: true,
		},
	}
}

// Load reads configuration from the given file (optional) with environment
// overrides (CYLON_* with nested keys joined by underscore) layered over the
// built-in defaults, then validates the result.
func Load(path string) (*Config, error) {
	v := viper.New()

	d := Default()
	v.SetDefault("server.http_listen", d.Server.HTTPListen)
	v.SetDefault("server.metrics_listen", d.Server.MetricsListen)
	v.SetDefault("server.log_level", d.Server.LogLevel)
	v.SetDefault("store.path", d.Store.Path)
	v.SetDefault("gateway.eui", d.Gateway.EUI)
	v.SetDefault("gateway.tcp_listen", d.Gateway.TCPListen)
	v.SetDefault("gateway.connection.creds_dir", d.Gateway.Connection.CredsDir)
	v.SetDefault("sim.realtime", d.Sim.Realtime)

	v.SetEnvPrefix("CYLON")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config %q: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decoding config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the configuration for obviously invalid values.
func (c *Config) Validate() error {
	if c.Server.HTTPListen == "" {
		return fmt.Errorf("server.http_listen must not be empty")
	}
	if !validLogLevels[strings.ToLower(c.Server.LogLevel)] {
		return fmt.Errorf("server.log_level %q is invalid (want debug|info|warning|error)", c.Server.LogLevel)
	}
	if c.Store.Path == "" {
		return fmt.Errorf("store.path must not be empty")
	}
	if c.Gateway.EUI != "" && !euiRe.MatchString(c.Gateway.EUI) {
		return fmt.Errorf("gateway.eui %q is invalid (want 16 hex chars)", c.Gateway.EUI)
	}
	if c.Gateway.EUIPrefix != "" && !euiPrefixRe.MatchString(c.Gateway.EUIPrefix) {
		return fmt.Errorf("gateway.eui_prefix %q is invalid (want even-length hex up to 16 chars)", c.Gateway.EUIPrefix)
	}
	return nil
}
