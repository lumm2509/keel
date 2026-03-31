package keel

import (
	"context"
	"errors"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lumm2509/keel/apis"
	"github.com/lumm2509/keel/commands"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/runtime/hook"
	transporthttp "github.com/lumm2509/keel/transport/http"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/acme/autocert"
)

var Version = "(untracked)"

// restartGracePeriod is the time the server waits after initiating a restart
// before signaling the WaitGroup done, allowing the new process to take over.
const restartGracePeriod = 3 * time.Second


type ServeConfig = apis.ServeConfig

type BootstrapEvent[T any] struct {
	hook.Event
	App *T
}

type TerminateEvent[T any] struct {
	hook.Event
	App       *T
	IsRestart bool
}

type ServeEvent[T any] struct {
	hook.Event
	App         *T
	Router      *transporthttp.Router[*transporthttp.RequestEvent[T]]
	Server      *stdhttp.Server
	CertManager *autocert.Manager
	Listener    net.Listener
}

// Config holds all options for creating a new App.
// Pass it directly to New: keel.New(keel.Config[MyApp]{...}).
type Config[T any] struct {
	// Context is the app-scoped value shared across all requests.
	// Mutually exclusive with ContextFactory.
	Context *T

	// ContextFactory creates a new T per request.
	// Mutually exclusive with Context.
	ContextFactory func(stdhttp.ResponseWriter, *stdhttp.Request) *T

	// Module is the shared configuration (logger, data dir, TLS, etc.).
	Module *config.Config

	// HMR enables Hot Module Reload in the develop command.
	HMR commands.HMRFunc

	// HideStartBanner suppresses the startup banner.
	HideStartBanner bool
}

type App[T any] struct {
	*transporthttp.Router[*transporthttp.RequestEvent[T]]

	context         *T
	contextFactory  func(stdhttp.ResponseWriter, *stdhttp.Request) *T
	config          *config.Config
	hmr             commands.HMRFunc
	hideStartBanner bool
	bootstrapped    bool
	onBootstrap     *hook.Hook[*BootstrapEvent[T]]
	onServe         *hook.Hook[*ServeEvent[T]]
	onTerminate     *hook.Hook[*TerminateEvent[T]]

	rootCmd *cobra.Command
}

// New creates a new App from the provided Config.
func New[T any](cfg Config[T]) *App[T] {
	executableName := filepath.Base(os.Args[0])

	if cfg.Module == nil {
		cfg.Module = &config.Config{}
	}

	rootCmd := &cobra.Command{
		Use:     executableName,
		Short:   executableName + " CLI",
		Version: Version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage: true,
	}

	app := &App[T]{
		context:         cfg.Context,
		contextFactory:  cfg.ContextFactory,
		config:          cfg.Module,
		hmr:             cfg.HMR,
		hideStartBanner: cfg.HideStartBanner,
		rootCmd:         rootCmd,
		onBootstrap:     &hook.Hook[*BootstrapEvent[T]]{},
		onServe:         &hook.Hook[*ServeEvent[T]]{},
		onTerminate:     &hook.Hook[*TerminateEvent[T]]{},
	}

	requestEventPool := sync.Pool{
		New: func() any {
			return &transporthttp.RequestEvent[T]{}
		},
	}

	app.Router = transporthttp.NewRouter(func(w stdhttp.ResponseWriter, r *stdhttp.Request) (*transporthttp.RequestEvent[T], transporthttp.EventCleanupFunc) {
		event := requestEventPool.Get().(*transporthttp.RequestEvent[T])

		ctx := app.context
		if app.contextFactory != nil {
			ctx = app.contextFactory(w, r)
		}
		event.Reset(ctx, w, r)

		return event, func() {
			event.Release()
			requestEventPool.Put(event)
		}
	})

	app.rootCmd.SetErr(commands.NewColoredErrWriter())
	app.rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return app
}

func (a *App[T]) Start() error {
	if len(os.Args) == 1 {
		a.rootCmd.SetArgs([]string{"develop"})
	}

	a.rootCmd.AddCommand(a.newDevelopCommand())
	return a.Execute()
}

func (a *App[T]) Execute() error {
	if !a.skipBootstrap() {
		if err := a.bootstrap(); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- a.rootCmd.ExecuteContext(ctx)
	}()

	var cmdErr error
	select {
	case cmdErr = <-cmdDone:
		stop()
		if cmdErr != nil {
			return cmdErr
		}
	case <-ctx.Done():
		stop()
		cmdErr = <-cmdDone // drain the goroutine before proceeding
		_ = cmdErr
	}

	return a.terminate(false)
}

func (a *App[T]) bootstrap() error {
	event := &BootstrapEvent[T]{App: a.context}

	err := a.OnBootstrap().Trigger(event, func(e *BootstrapEvent[T]) error {
		return e.Next()
	})
	if err == nil {
		a.bootstrapped = true
	}

	return err
}

func (a *App[T]) terminate(isRestart bool) error {
	event := &TerminateEvent[T]{
		App:       a.context,
		IsRestart: isRestart,
	}

	return a.OnTerminate().Trigger(event, func(e *TerminateEvent[T]) error {
		return e.Next()
	})
}

func (a *App[T]) skipBootstrap() bool {
	if a.bootstrapped {
		return true
	}

	args := os.Args[1:]
	cmd, _, err := a.rootCmd.Find(args)
	if err != nil || cmd == nil {
		return false
	}

	for _, arg := range args {
		// Only consider the four standard help/version flags.
		var name, short string
		switch arg {
		case "--help":
			name = "help"
		case "-h":
			short = "h"
		case "--version":
			name = "version"
		case "-v":
			short = "v"
		default:
			continue
		}

		// If the flag is NOT registered as a local flag on the resolved command,
		// it is Cobra's own global help/version flag → skip bootstrap.
		if name != "" && cmd.Flags().Lookup(name) == nil {
			return true
		}
		if short != "" && cmd.Flags().ShorthandLookup(short) == nil {
			return true
		}
	}

	return false
}

func (a *App[T]) OnBootstrap() *hook.Hook[*BootstrapEvent[T]] {
	return a.onBootstrap
}

func (a *App[T]) OnServe() *hook.Hook[*ServeEvent[T]] {
	return a.onServe
}

func (a *App[T]) OnTerminate() *hook.Hook[*TerminateEvent[T]] {
	return a.onTerminate
}

// Context returns the app-scoped T provided via Config.Context.
func (a *App[T]) Context() *T {
	return a.context
}

func (a *App[T]) Config() *config.Config {
	return a.config
}

func (a *App[T]) newDevelopCommand() *cobra.Command {
	return commands.NewDevelopCommand(
		a.hmr,
		!a.hideStartBanner,
		func(cfg apis.ServeConfig) error {
			err := a.serve(cfg)
			if errors.Is(err, stdhttp.ErrServerClosed) {
				return nil
			}
			return err
		},
	)
}

func (a *App[T]) serve(config ServeConfig) error {
	prepared, err := apis.PrepareServe(a.context, a.config, config, a.Router)
	if err != nil {
		return err
	}
	defer prepared.Close()
	defer a.OnTerminate().Unbind("keelGracefulShutdown")

	var listener net.Listener
	var wg sync.WaitGroup

	a.OnTerminate().Bind(&hook.Handler[*TerminateEvent[T]]{
		Id: "keelGracefulShutdown",
		Func: func(te *TerminateEvent[T]) error {
			prepared.Close()

			shutdownTimeout := config.ShutdownTimeout
			if shutdownTimeout <= 0 {
				shutdownTimeout = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			wg.Add(1)
			if err := prepared.Server.Shutdown(ctx); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
				a.config.ResolveLogger().Error("graceful shutdown incomplete, some connections were forcibly closed", "error", err)
			}

			if te.IsRestart {
				time.AfterFunc(restartGracePeriod, func() { wg.Done() })
			} else {
				wg.Done()
			}

			return te.Next()
		},
		Priority: -9999,
	})

	defer func() {
		wg.Wait()
		if listener != nil && listener != prepared.Listener {
			_ = listener.Close()
		}
	}()

	serveEvent := &ServeEvent[T]{
		App:         a.context,
		Router:      prepared.Router,
		Server:      prepared.Server,
		CertManager: prepared.CertManager,
		Listener:    prepared.Listener,
	}

	if err := a.OnServe().Trigger(serveEvent, func(e *ServeEvent[T]) error {
		return e.Next()
	}); err != nil {
		return err
	}

	prepared.Router = serveEvent.Router
	prepared.Server = serveEvent.Server
	prepared.CertManager = serveEvent.CertManager
	prepared.Listener = serveEvent.Listener
	listener = prepared.Listener

	if listener == nil {
		return errors.New("the OnServe listener was not initialized; did you forget to call e.Next()?")
	}

	baseURL := prepared.BaseURL
	if prepared.Server != nil {
		baseURL = apis.BaseURL(config, prepared.Server.Addr)
	}

	if config.ShowStartBanner {
		apis.StartBanner(baseURL)
	}

	if config.HttpsAddr != "" {
		if config.HttpAddr != "" && prepared.CertManager != nil {
			go func() {
				if err := stdhttp.ListenAndServe(config.HttpAddr, prepared.CertManager.HTTPHandler(nil)); err != nil {
					a.config.ResolveLogger().Error("HTTP redirect listener failed", "addr", config.HttpAddr, "error", err)
				}
			}()
		}
		err = prepared.Server.ServeTLS(listener, "", "")
	} else {
		err = prepared.Server.Serve(listener)
	}

	if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		return err
	}

	return nil
}

