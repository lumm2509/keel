package config

import (
	"encoding/json"
	"testing"
)

func TestConfigModuleAutoCertRoundTrip(t *testing.T) {
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

	var cfg ConfigModule
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

func TestConfigModuleZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	var cfg ConfigModule

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
