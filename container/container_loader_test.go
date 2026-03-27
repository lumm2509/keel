package container

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/lumm2509/keel/config"
)

func TestLoadBasecontainerUsesProvidedCradle(t *testing.T) {
	type cradle struct {
		Name string
	}

	cfg := &config.ConfigModule{}
	expected := &cradle{Name: "test"}

	container := LoadBasecontainer(cfg, expected)

	if container.Cradle() != expected {
		t.Fatalf("expected cradle pointer %p, got %p", expected, container.Cradle())
	}

	if container.Cradle().Name != "test" {
		t.Fatalf("expected cradle name %q, got %q", "test", container.Cradle().Name)
	}
}

func TestLoadBasecontainerInitializesZeroCradle(t *testing.T) {
	type cradle struct {
		Count int
	}

	container := LoadBasecontainer[cradle](&config.ConfigModule{})

	if container.Cradle() == nil {
		t.Fatalf("expected cradle to be initialized")
	}

	if container.Cradle().Count != 0 {
		t.Fatalf("expected zero-value cradle count %d, got %d", 0, container.Cradle().Count)
	}
}

type testSQLDriver struct {
	closeCalls *int32
}

type testSQLConn struct {
	closeCalls *int32
}

func (d *testSQLDriver) Open(string) (driver.Conn, error) {
	return &testSQLConn{closeCalls: d.closeCalls}, nil
}

func (c *testSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *testSQLConn) Close() error {
	atomic.AddInt32(c.closeCalls, 1)
	return nil
}

func (c *testSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *testSQLConn) Ping(context.Context) error {
	return nil
}

func TestInitResourcesInitializesDatabaseAndExposesIt(t *testing.T) {
	type cradle struct{}

	cfg := &config.ConfigModule{}
	container := LoadBasecontainer[cradle](cfg)

	var closeCalls int32
	driverName := "container_test_bootstrap"
	sql.Register(driverName, &testSQLDriver{closeCalls: &closeCalls})

	container.dbConnect = func() (*sql.DB, error) {
		db, err := sql.Open(driverName, "postgres://bootstrap-test")
		if err != nil {
			return nil, err
		}

		if err := db.Ping(); err != nil {
			return nil, err
		}

		return db, nil
	}
	if err := container.InitResources(); err != nil {
		t.Fatalf("InitResources() error = %v", err)
	}

	if container.DataBase() == nil {
		t.Fatalf("expected database to be initialized")
	}

	if !container.ResourcesReady() {
		t.Fatalf("expected container resources to be initialized")
	}

	if err := container.ResetResources(); err != nil {
		t.Fatalf("ResetResources() error = %v", err)
	}

	if container.DataBase() != nil {
		t.Fatalf("expected database to be cleared after reset")
	}

	if atomic.LoadInt32(&closeCalls) == 0 {
		t.Fatalf("expected underlying sql connection to be closed")
	}
}

func TestInitResourcesWithoutDatabaseURLKeepsContainerUsable(t *testing.T) {
	type cradle struct{}

	container := LoadBasecontainer[cradle](&config.ConfigModule{})

	err := container.InitResources()
	if err != nil {
		t.Fatalf("expected InitResources() to succeed without database url, got %v", err)
	}

	if !container.ResourcesReady() {
		t.Fatalf("expected container resources to initialize without database url")
	}

	if container.DataBase() != nil {
		t.Fatalf("expected database to remain nil when no database url is configured")
	}
}

func TestInitResourcesWithoutConfigKeepsContainerUsable(t *testing.T) {
	type cradle struct{}

	container := LoadBasecontainer[cradle](nil)

	if err := container.InitResources(); err != nil {
		t.Fatalf("expected InitResources() to succeed without config, got %v", err)
	}

	if !container.ResourcesReady() {
		t.Fatalf("expected container resources to initialize without config")
	}

	if container.DataBase() != nil {
		t.Fatalf("expected database to remain nil when config is missing")
	}
}

func TestBaseContainerOptionalCapabilitiesReturnExplicitDefaults(t *testing.T) {
	type cradle struct{}

	container := LoadBasecontainer[cradle](&config.ConfigModule{})

	if got := container.DataDir(); got != "" {
		t.Fatalf("expected empty data dir by default, got %q", got)
	}

	if got := container.EncryptionEnv(); got != "" {
		t.Fatalf("expected empty encryption env by default, got %q", got)
	}

	fs, err := container.NewFilesystem()
	if err == nil {
		t.Fatalf("expected NewFilesystem() to fail without configured data dir, got fs=%v", fs)
	}

	if !errors.Is(err, errDataDirNotConfigured) {
		t.Fatalf("expected errDataDirNotConfigured, got %v", err)
	}

	if err := container.ReloadSettings(); !errors.Is(err, errContainerReloadNotImplemented) {
		t.Fatalf("expected errContainerReloadNotImplemented, got %v", err)
	}

	if err := container.Restart(); !errors.Is(err, errContainerRestartNotImplemented) {
		t.Fatalf("expected errContainerRestartNotImplemented, got %v", err)
	}
}

func ptr[T any](v T) *T {
	return &v
}
