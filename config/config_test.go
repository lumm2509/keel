package config

import (
	"encoding/json"
	"testing"
)

func TestConfigModuleJSONTrustedProxyCIDRsRoundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"projectConfig": {
			"isDev": true,
			"http": {
				"trustedProxyCidrs": ["10.0.0.0/8", "192.168.0.0/16"],
				"allowedOrigins": ["https://example.com"]
			}
		},
		"featureFlags": {"alpha": true}
	}`)

	var cfg ConfigModule
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !cfg.Projectconfig.IsDev {
		t.Fatalf("IsDev = false, want true")
	}
	if cfg.Projectconfig.Http == nil {
		t.Fatalf("Http config = nil, want populated")
	}
	if len(cfg.Projectconfig.Http.TrustedProxyCIDRs) != 2 {
		t.Fatalf("TrustedProxyCIDRs len = %d, want 2", len(cfg.Projectconfig.Http.TrustedProxyCIDRs))
	}
	if cfg.Projectconfig.Http.TrustedProxyCIDRs[0] != "10.0.0.0/8" || cfg.Projectconfig.Http.TrustedProxyCIDRs[1] != "192.168.0.0/16" {
		t.Fatalf("TrustedProxyCIDRs = %#v, want exact round trip", cfg.Projectconfig.Http.TrustedProxyCIDRs)
	}
}

func TestProjectConfigOptionsZeroValuesRemainUsable(t *testing.T) {
	t.Parallel()

	var cfg ConfigModule
	if cfg.Projectconfig.Http != nil {
		t.Fatalf("Http config = %#v, want nil by default", cfg.Projectconfig.Http)
	}
	if cfg.FeatureFlags != nil {
		t.Fatalf("FeatureFlags = %#v, want nil by default", cfg.FeatureFlags)
	}
}

func TestWorkerModesStableValues(t *testing.T) {
	t.Parallel()

	if Shared != "shared" {
		t.Fatalf("Shared = %q, want %q", Shared, "shared")
	}
	if Worker != "worker" {
		t.Fatalf("Worker = %q, want %q", Worker, "worker")
	}
	if Server != "server" {
		t.Fatalf("Server = %q, want %q", Server, "server")
	}
}
