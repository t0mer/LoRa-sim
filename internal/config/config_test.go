package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if cfg.Server.HTTPListen != ":8080" {
		t.Errorf("HTTPListen = %q, want :8080", cfg.Server.HTTPListen)
	}
	if cfg.Server.MetricsListen != ":9100" {
		t.Errorf("MetricsListen = %q, want :9100", cfg.Server.MetricsListen)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.Server.LogLevel)
	}
	if cfg.Store.Path != "/var/lib/cylon/cylon.db" {
		t.Errorf("Store.Path = %q, want /var/lib/cylon/cylon.db", cfg.Store.Path)
	}
	if cfg.Gateway.TCPListen != ":6000" {
		t.Errorf("Gateway.TCPListen = %q, want :6000", cfg.Gateway.TCPListen)
	}
	if cfg.Gateway.Connection.CredsDir != "/etc/cylon/creds" {
		t.Errorf("CredsDir = %q, want /etc/cylon/creds", cfg.Gateway.Connection.CredsDir)
	}
	if !cfg.Sim.Realtime {
		t.Errorf("Sim.Realtime = false, want true")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cylon.yaml")
	yaml := `
server:
  http_listen: ":9999"
  log_level: debug
store:
  path: /tmp/cylon.db
gateway:
  eui: "0102030405060708"
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	if cfg.Server.HTTPListen != ":9999" {
		t.Errorf("HTTPListen = %q, want :9999", cfg.Server.HTTPListen)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.Server.LogLevel)
	}
	if cfg.Store.Path != "/tmp/cylon.db" {
		t.Errorf("Store.Path = %q, want /tmp/cylon.db", cfg.Store.Path)
	}
	if cfg.Gateway.EUI != "0102030405060708" {
		t.Errorf("Gateway.EUI = %q, want 0102030405060708", cfg.Gateway.EUI)
	}
}

func TestEnvOverridesFileAndDefault(t *testing.T) {
	t.Setenv("CYLON_SERVER_HTTP_LISTEN", ":7777")
	t.Setenv("CYLON_GATEWAY_EUI", "aabbccddeeff0011")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPListen != ":7777" {
		t.Errorf("HTTPListen = %q, want :7777 (env override)", cfg.Server.HTTPListen)
	}
	if cfg.Gateway.EUI != "aabbccddeeff0011" {
		t.Errorf("Gateway.EUI = %q, want aabbccddeeff0011 (env override)", cfg.Gateway.EUI)
	}
}

func TestValidateRejectsBadLogLevel(t *testing.T) {
	cfg := Default()
	cfg.Server.LogLevel = "loud"
	if err := cfg.Validate(); err == nil {
		t.Errorf("Validate() = nil, want error for bad log level")
	}
}

func TestValidateRejectsBadEUI(t *testing.T) {
	cfg := Default()
	cfg.Gateway.EUI = "xyz"
	if err := cfg.Validate(); err == nil {
		t.Errorf("Validate() = nil, want error for bad EUI")
	}
}

func TestValidateAcceptsEmptyEUI(t *testing.T) {
	cfg := Default()
	cfg.Gateway.EUI = ""
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for empty EUI (generate on first run)", err)
	}
}
