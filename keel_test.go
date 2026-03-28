package keel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lumm2509/keel/config"
)

func TestNewWithCradleBuildsDefaultContainer(t *testing.T) {
	t.Parallel()

	type cradle struct {
		Name string
	}

	app := New(WithCradle(cradle{Name: "demo"}))

	if app.container == nil {
		t.Fatalf("expected default container to be created")
	}

	if got := app.container.Cradle().Name; got != "demo" {
		t.Fatalf("expected cradle name %q, got %q", "demo", got)
	}
}

func TestNewInitializesDefaultDataDir(t *testing.T) {
	app := New[struct{}]()

	want := filepath.Join(mustGetwd(t), "pb_data")
	if got := app.Container().DataDir(); got != want {
		t.Fatalf("expected data dir %q, got %q", want, got)
	}
}

func TestNewUsesConfigModuleDataDirAndEncryptionEnv(t *testing.T) {
	dataDir := "/tmp/keel-config-data"
	encryptionEnv := "KEEL_SECRET"

	app := New[struct{}](WithConfig[struct{}](&config.ConfigModule{
		Projectconfig: config.ProjectConfigOptions{
			DataDir:       &dataDir,
			EncryptionEnv: &encryptionEnv,
		},
	}))

	if got := app.Container().DataDir(); got != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, got)
	}

	if got := app.Container().EncryptionEnv(); got != encryptionEnv {
		t.Fatalf("expected encryption env %q, got %q", encryptionEnv, got)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	return wd
}
