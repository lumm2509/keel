package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/lumm2509/keel/apis"
)

func TestDevelopCommandRequiresServe(t *testing.T) {
	t.Parallel()

	cmd := NewDevelopCommand(nil, true, nil)
	cmd.SetArgs(nil)

	err := cmd.ExecuteContext(context.Background())
	if err == nil || err.Error() != "develop command requires a serve function" {
		t.Fatalf("expected missing serve error, got %v", err)
	}
}

func TestDevelopCommandDefaultHTTPAddrWithoutDomains(t *testing.T) {
	t.Parallel()

	var got apis.ServeConfig
	cmd := NewDevelopCommand(nil, true, func(cfg apis.ServeConfig) error {
		got = cfg
		return nil
	})
	cmd.SetArgs(nil)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if got.HttpAddr != "127.0.0.1:8090" {
		t.Fatalf("HttpAddr = %q, want %q", got.HttpAddr, "127.0.0.1:8090")
	}
	if got.HttpsAddr != "" {
		t.Fatalf("HttpsAddr = %q, want empty", got.HttpsAddr)
	}
	if !got.ShowStartBanner {
		t.Fatalf("ShowStartBanner = false, want true")
	}
}

func TestDevelopCommandDefaultsTLSAddressesWithDomains(t *testing.T) {
	t.Parallel()

	var got apis.ServeConfig
	cmd := NewDevelopCommand(nil, false, func(cfg apis.ServeConfig) error {
		got = cfg
		return nil
	})
	cmd.SetArgs([]string{"example.com", "api.example.com"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if got.HttpAddr != "0.0.0.0:80" {
		t.Fatalf("HttpAddr = %q, want %q", got.HttpAddr, "0.0.0.0:80")
	}
	if got.HttpsAddr != "0.0.0.0:443" {
		t.Fatalf("HttpsAddr = %q, want %q", got.HttpsAddr, "0.0.0.0:443")
	}
	if got.ShowStartBanner {
		t.Fatalf("ShowStartBanner = true, want false")
	}
	if len(got.CertificateDomains) != 2 || got.CertificateDomains[0] != "example.com" || got.CertificateDomains[1] != "api.example.com" {
		t.Fatalf("CertificateDomains = %#v, want both domains", got.CertificateDomains)
	}
}

func TestDevelopCommandPropagatesHMRErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("hmr failed")
	cmd := NewDevelopCommand(func(context.Context) error {
		return want
	}, true, func(apis.ServeConfig) error {
		return nil
	})

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("expected HMR error, got %v", err)
	}
}
