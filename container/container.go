package container

import (
	"database/sql"
	"log/slog"

	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/infra/filesystem"
	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/subscriptions"
	"github.com/lumm2509/keel/runtime/cron"
	"github.com/lumm2509/keel/runtime/hook"
)

type Container[Cradle any] interface {
	Logger() *slog.Logger
	Cradle() *Cradle
	IsBootstrapped() bool
	Bootstrap() error
	ResetBootstrapState() error
	DataDir() string
	EncryptionEnv() string
	IsDev() bool
	Config() *config.ConfigModule
	Store() *store.Store[string, any]
	Cron() *cron.Cron
	SubscriptionsBroker() *subscriptions.Broker
	NewFilesystem() (*filesystem.System, error)
	ReloadSettings() error
	Restart() error
	DataBase() *sql.DB

	// ---------------------------------------------------------------
	// App event hooks
	// ---------------------------------------------------------------

	// OnBootstrap hook is triggered when initializing the main application
	// resources (db, app settings, etc).
	OnBootstrap() *hook.Hook[*BootstrapEvent[Cradle]]

	// OnServe hook is triggered when the app web server is started
	// (after starting the TCP listener but before initializing the blocking serve task),
	// allowing you to adjust its options and attach new routes or middlewares.
	OnServe() *hook.Hook[*ServeEvent[Cradle]]

	// OnTerminate hook is triggered when the app is in the process
	// of being terminated (ex. on SIGTERM signal).
	//
	// Note that the app could be terminated abruptly without awaiting the hook completion.
	OnTerminate() *hook.Hook[*TerminateEvent[Cradle]]
}
