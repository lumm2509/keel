package container

import (
	"database/sql"
	"log/slog"
	"net/netip"

	"github.com/lumm2509/keel/infra/filesystem"
	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/subscriptions"
	"github.com/lumm2509/keel/runtime/cron"
)

type Container[Cradle any] interface {
	Logger() *slog.Logger
	Cradle() *Cradle
	ResourcesReady() bool
	InitResources() error
	ResetResources() error
	IsDev() bool
	Store() *store.Store[string, any]
	Cron() *cron.Cron
	SubscriptionsBroker() *subscriptions.Broker
	DataBase() *sql.DB
}

type DataDirProvider interface{ DataDir() string }
type EncryptionEnvProvider interface{ EncryptionEnv() string }
type FilesystemProvider interface {
	NewFilesystem() (*filesystem.System, error)
}
type SettingsReloader interface{ ReloadSettings() error }
type Restarter interface{ Restart() error }
type TrustedProxyProvider interface{ TrustedProxyRanges() []netip.Prefix }
