package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const DefaultPingTimeout = 5 * time.Second
const DefaultRetryDelay = 250 * time.Millisecond

var ErrMissingURL = errors.New("database url is required")

type Options struct {
	URL             string
	PoolMin         int
	PoolMax         int
	IdleTimeout     time.Duration
	ConnMaxLifetime time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
}

type OpenFunc func(driverName string, dataSourceName string) (*sql.DB, error)

type Connector struct {
	open OpenFunc
}

func NewConnector() *Connector {
	return &Connector{open: sql.Open}
}

func Open(options Options) (*sql.DB, error) {
	return NewConnector().Open(options)
}

func (c *Connector) Open(options Options) (*sql.DB, error) {
	if options.URL == "" {
		return nil, ErrMissingURL
	}

	db, err := c.open("pgx", options.URL)
	if err != nil {
		return nil, err
	}

	applyPoolOptions(db, options)

	if err := pingWithRetry(db, options); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func applyPoolOptions(db *sql.DB, options Options) {
	if options.PoolMax > 0 {
		db.SetMaxOpenConns(options.PoolMax)
	}

	if options.PoolMin > 0 {
		db.SetMaxIdleConns(options.PoolMin)
	}

	if options.IdleTimeout > 0 {
		db.SetConnMaxIdleTime(options.IdleTimeout)
	}

	if options.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(options.ConnMaxLifetime)
	}
}

func pingWithRetry(db *sql.DB, options Options) error {
	retryDelay := options.RetryDelay
	if retryDelay <= 0 {
		retryDelay = DefaultRetryDelay
	}

	var lastErr error

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
		lastErr = db.PingContext(ctx)
		cancel()

		if lastErr == nil {
			return nil
		}

		if attempt < options.MaxRetries {
			time.Sleep(retryDelay)
		}
	}

	return lastErr
}
