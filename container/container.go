package container

import (
	"database/sql"
	"log/slog"
	"net/netip"

	"github.com/lumm2509/keel/dal"
	"github.com/lumm2509/keel/dml"
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
	DataDir() string
	EncryptionEnv() string
	ReloadSettings() error
	Store() *store.Store[string, any]
	Cron() *cron.Cron
	SubscriptionsBroker() *subscriptions.Broker
	DataBase() *sql.DB
	Dal() *dal.Service
	Dml() *dml.Service
}

// DataDirProvider is implemented by containers that expose a data directory path.
// Used by the HTTP serve layer to locate TLS certificate cache directories.
type DataDirProvider interface{ DataDir() string }

// TrustedProxyProvider is implemented by containers that declare trusted proxy CIDR ranges.
type TrustedProxyProvider interface{ TrustedProxyRanges() []netip.Prefix }
