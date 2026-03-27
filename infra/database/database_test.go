package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
)

type testDriver struct {
	closeCalls *int32
}

type testConn struct {
	closeCalls *int32
}

func (d *testDriver) Open(string) (driver.Conn, error) {
	return &testConn{closeCalls: d.closeCalls}, nil
}

func (c *testConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *testConn) Close() error {
	atomic.AddInt32(c.closeCalls, 1)
	return nil
}

func (c *testConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *testConn) Ping(context.Context) error {
	return nil
}

func TestOpenRequiresDatabaseURL(t *testing.T) {
	connector := NewConnector()

	_, err := connector.Open(Options{})
	if !errors.Is(err, ErrMissingURL) {
		t.Fatalf("expected error %v, got %v", ErrMissingURL, err)
	}
}

func TestOpenAppliesPoolOptions(t *testing.T) {
	var closeCalls int32
	driverName := "infra_database_test"
	sql.Register(driverName, &testDriver{closeCalls: &closeCalls})

	connector := &Connector{
		open: func(driverNameArg string, dataSourceName string) (*sql.DB, error) {
			if driverNameArg != "pgx" {
				t.Fatalf("expected driver %q, got %q", "pgx", driverNameArg)
			}

			if dataSourceName != "postgres://infra-test" {
				t.Fatalf("expected dsn %q, got %q", "postgres://infra-test", dataSourceName)
			}

			return sql.Open(driverName, dataSourceName)
		},
	}

	options := Options{
		URL:             "postgres://infra-test",
		PoolMin:         2,
		PoolMax:         5,
		IdleTimeout:     10,
		ConnMaxLifetime: 20,
	}

	db, err := connector.Open(options)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Fatalf("expected max open conns %d, got %d", 5, stats.MaxOpenConnections)
	}
}
