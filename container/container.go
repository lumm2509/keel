package container

import (
	"database/sql"
	"log/slog"

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
	DataDir() string
	EncryptionEnv() string
	IsDev() bool
	Store() *store.Store[string, any]
	Cron() *cron.Cron
	SubscriptionsBroker() *subscriptions.Broker
	NewFilesystem() (*filesystem.System, error)
	ReloadSettings() error
	Restart() error
	DataBase() *sql.DB
}
