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

func TestBootstrapInitializesDatabaseAndExposesIt(t *testing.T) {
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
	if err := container.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if container.DataBase() == nil {
		t.Fatalf("expected database to be initialized")
	}

	if !container.IsBootstrapped() {
		t.Fatalf("expected container to be bootstrapped")
	}

	if err := container.ResetBootstrapState(); err != nil {
		t.Fatalf("ResetBootstrapState() error = %v", err)
	}

	if container.DataBase() != nil {
		t.Fatalf("expected database to be cleared after reset")
	}

	if atomic.LoadInt32(&closeCalls) == 0 {
		t.Fatalf("expected underlying sql connection to be closed")
	}
}

func TestBootstrapFailsWithoutDatabaseURL(t *testing.T) {
	type cradle struct{}

	container := LoadBasecontainer[cradle](&config.ConfigModule{})

	err := container.Bootstrap()
	if err == nil {
		t.Fatalf("expected Bootstrap() to fail without database url")
	}
}

func ptr[T any](v T) *T {
	return &v
}
