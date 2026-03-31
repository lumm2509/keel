package config

import (
	"encoding/json"
	"log/slog"
	"testing"
)

func TestConfigAutoCertRoundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"dataDir": "/var/data",
		"http": {
			"autoCert": {
				"email": "admin@example.com",
				"hostWhitelist": ["example.com"]
			}
		}
	}`)

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if cfg.DataDir == nil || *cfg.DataDir != "/var/data" {
		t.Fatalf("DataDir = %v, want /var/data", cfg.DataDir)
	}
	if cfg.Http == nil || cfg.Http.AutoCert == nil {
		t.Fatalf("Http.AutoCert = nil, want populated")
	}
	if cfg.Http.AutoCert.Email == nil || *cfg.Http.AutoCert.Email != "admin@example.com" {
		t.Fatalf("Email = %v, want admin@example.com", cfg.Http.AutoCert.Email)
	}
	if len(cfg.Http.AutoCert.HostWhitelist) != 1 || cfg.Http.AutoCert.HostWhitelist[0] != "example.com" {
		t.Fatalf("HostWhitelist = %v, want [example.com]", cfg.Http.AutoCert.HostWhitelist)
	}
}

func TestConfigZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	var cfg Config

	if cfg.Http != nil {
		t.Fatalf("Http = %#v, want nil by default", cfg.Http)
	}
	if cfg.DataDir != nil {
		t.Fatalf("DataDir = %v, want nil by default", cfg.DataDir)
	}
	if cfg.Logger != nil {
		t.Fatalf("Logger = %v, want nil by default", cfg.Logger)
	}
}

func TestResolveLoggerNilConfigReturnsSlogDefault(t *testing.T) {
	t.Parallel()

	var cfg *Config
	if got := cfg.ResolveLogger(); got != slog.Default() {
		t.Fatalf("nil Config.ResolveLogger() should return slog.Default()")
	}
}

func TestResolveLoggerNilLoggerFieldReturnsSlogDefault(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	if got := cfg.ResolveLogger(); got != slog.Default() {
		t.Fatalf("Config{}.ResolveLogger() should return slog.Default() when Logger is nil")
	}
}

func TestResolveLoggerReturnsCustomLogger(t *testing.T) {
	t.Parallel()

	custom := slog.New(slog.NewTextHandler(nil, nil))
	cfg := &Config{Logger: custom}
	if got := cfg.ResolveLogger(); got != custom {
		t.Fatalf("expected custom logger, got %v", got)
	}
}
