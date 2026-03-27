package container

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/netip"
	"time"

	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/infra/database"
	"github.com/lumm2509/keel/infra/filesystem"
	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/subscriptions"
	"github.com/lumm2509/keel/runtime/cron"
)

var _ Container[any] = (*BaseContainer[any])(nil)

var errDataDirNotConfigured = errors.New("container data dir is not configured")
var errContainerReloadNotImplemented = errors.New("container reload is not implemented")
var errContainerRestartNotImplemented = errors.New("container restart is not implemented")

type BaseContainer[Cradle any] struct {
	config              *config.ConfigModule
	cradle              *Cradle
	bootstrapped        bool
	db                  *sql.DB
	store               *store.Store[string, any]
	cron                *cron.Cron
	subscriptionsBroker *subscriptions.Broker
	logger              *slog.Logger
	dbConnect           func() (*sql.DB, error)
}

func LoadBasecontainer[C any](config *config.ConfigModule, cradle ...*C) *BaseContainer[C] {
	containerCradle := new(C)
	if len(cradle) > 0 && cradle[0] != nil {
		containerCradle = cradle[0]
	}

	container := &BaseContainer[C]{
		config:              config,
		cradle:              containerCradle,
		store:               store.New[string, any](nil),
		cron:                cron.New(),
		subscriptionsBroker: subscriptions.NewBroker(),
	}
	container.dbConnect = container.connectDataDB

	return container
}

// Cradle implements [Container].
func (b *BaseContainer[Cradle]) Cradle() *Cradle {
	return b.cradle
}

// Cron implements [Container].
func (b *BaseContainer[Cradle]) Cron() *cron.Cron {
	return b.cron
}

// DataBase implements [Container].
func (b *BaseContainer[Cradle]) DataBase() *sql.DB {
	return b.db
}

// DataDir implements [Container].
func (b *BaseContainer[Cradle]) DataDir() string {
	return ""
}

// EncryptionEnv implements [Container].
func (b *BaseContainer[Cradle]) EncryptionEnv() string {
	return ""
}

// ResourcesReady implements [Container].
func (b *BaseContainer[Cradle]) ResourcesReady() bool {
	return b.bootstrapped
}

// IsDev implements [Container].
func (b *BaseContainer[Cradle]) IsDev() bool {
	return b.config != nil && b.config.Projectconfig.IsDev
}

// Logger implements [Container].
func (b *BaseContainer[Cradle]) Logger() *slog.Logger {
	if b.logger != nil {
		return b.logger
	}

	if b.config != nil && b.config.Logger != nil {
		return b.config.Logger
	}

	return slog.Default()
}

// NewFilesystem implements [Container].
func (b *BaseContainer[Cradle]) NewFilesystem() (*filesystem.System, error) {
	dataDir := b.DataDir()
	if dataDir == "" {
		return nil, errDataDirNotConfigured
	}

	return filesystem.NewLocal(dataDir)
}

// ReloadSettings implements [Container].
func (b *BaseContainer[Cradle]) ReloadSettings() error {
	return errContainerReloadNotImplemented
}

// ResetResources implements [Container].
func (b *BaseContainer[Cradle]) ResetResources() error {
	b.Cron().Stop()
	b.bootstrapped = false

	if b.db == nil {
		return nil
	}

	err := b.db.Close()
	b.db = nil

	return err
}

// InitResources implements [Container].
func (b *BaseContainer[Cradle]) InitResources() error {
	if err := b.ResetResources(); err != nil {
		return err
	}

	if err := b.initLogger(); err != nil {
		return err
	}

	if err := b.initDataDB(); err != nil {
		return err
	}

	b.bootstrapped = true
	return nil
}

// Restart implements [Container].
func (b *BaseContainer[Cradle]) Restart() error {
	return errContainerRestartNotImplemented
}

// Store implements [Container].
func (b *BaseContainer[Cradle]) Store() *store.Store[string, any] {
	return b.store
}

// SubscriptionsBroker implements [Container].
func (b *BaseContainer[Cradle]) SubscriptionsBroker() *subscriptions.Broker {
	return b.subscriptionsBroker
}

func (b *BaseContainer[Cradle]) initLogger() error {
	if b.config != nil && b.config.Logger != nil {
		b.logger = b.config.Logger
		return nil
	}

	b.logger = slog.Default()
	return nil
}

func (b *BaseContainer[Cradle]) initDataDB() error {
	db, err := b.dbConnect()
	if err != nil {
		return err
	}

	b.db = db
	return nil
}

func (b *BaseContainer[Cradle]) connectDataDB() (*sql.DB, error) {
	if b.config == nil {
		return nil, nil
	}

	options := database.Options{}

	if b.config.Projectconfig.DatabaseUrl != nil {
		options.URL = *b.config.Projectconfig.DatabaseUrl
	}

	if options.URL == "" {
		return nil, nil
	}

	if driverOptions := b.config.Projectconfig.DatabaseDriverOptions; driverOptions != nil {
		if driverOptions.PoolMin != nil {
			options.PoolMin = *driverOptions.PoolMin
		}

		if driverOptions.PoolMax != nil {
			options.PoolMax = *driverOptions.PoolMax
		}

		if driverOptions.IdleTimeoutMillis != nil {
			options.IdleTimeout = time.Duration(*driverOptions.IdleTimeoutMillis) * time.Millisecond
		}

		if driverOptions.ConnMaxLifetimeMs != nil {
			options.ConnMaxLifetime = time.Duration(*driverOptions.ConnMaxLifetimeMs) * time.Millisecond
		}

		if driverOptions.MaxRetries != nil {
			options.MaxRetries = *driverOptions.MaxRetries
		}

		if driverOptions.RetryDelayMs != nil {
			options.RetryDelay = time.Duration(*driverOptions.RetryDelayMs) * time.Millisecond
		}
	}

	return database.Open(options)
}

func (b *BaseContainer[Cradle]) TrustedProxyRanges() []netip.Prefix {
	if b.config == nil || b.config.Projectconfig.Http == nil {
		return nil
	}

	result := make([]netip.Prefix, 0, len(b.config.Projectconfig.Http.TrustedProxyCIDRs))
	for _, raw := range b.config.Projectconfig.Http.TrustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(raw)
		if err == nil {
			result = append(result, prefix)
		}
	}

	return result
}
