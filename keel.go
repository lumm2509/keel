package keel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/lumm2509/keel/apis"
	"github.com/lumm2509/keel/commands"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/runtime/hook"
	transporthttp "github.com/lumm2509/keel/transport/http"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/acme/autocert"
)

var Version = "(untracked)"

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

type Config[T any] struct {
	Context         *T
	ContextFactory  func(stdhttp.ResponseWriter, *stdhttp.Request) *T
	Module          *config.ConfigModule
	HMR             commands.HMRFunc
	HideStartBanner bool
}

func (cfg Config[T]) apply(b *builderConfig[T]) {
	if cfg.Context != nil {
		b.context = cfg.Context
	}
	if cfg.ContextFactory != nil {
		b.contextFactory = cfg.ContextFactory
	}
	if cfg.Module != nil {
		b.config = cfg.Module
	}
	b.hmr = cfg.HMR
	b.hideStartBanner = cfg.HideStartBanner
}

type App[T any] struct {
	*transporthttp.Router[*transporthttp.RequestEvent[T]]

	context         *T
	contextFactory  func(stdhttp.ResponseWriter, *stdhttp.Request) *T
	config          *config.ConfigModule
	hmr             commands.HMRFunc
	hideStartBanner bool
	bootstrapped    bool
	onBootstrap     *hook.Hook[*BootstrapEvent[T]]
	onServe         *hook.Hook[*ServeEvent[T]]
	onTerminate     *hook.Hook[*TerminateEvent[T]]

	rootCmd *cobra.Command
}

type Option[T any] interface {
	apply(*builderConfig[T])
}

type builderConfig[T any] struct {
	context         *T
	contextFactory  func(stdhttp.ResponseWriter, *stdhttp.Request) *T
	config          *config.ConfigModule
	hmr             commands.HMRFunc
	hideStartBanner bool
}

type optionFunc[T any] func(*builderConfig[T])

func (fn optionFunc[T]) apply(cfg *builderConfig[T]) {
	fn(cfg)
}

// WithContext sets the app-scoped context that will be shared across all requests.
func WithContext[T any](ctx *T) Option[T] {
	return optionFunc[T](func(cfg *builderConfig[T]) {
		cfg.context = ctx
	})
}

// WithContextFactory sets a per-request factory that creates a new T for each request.
func WithContextFactory[T any](fn func(stdhttp.ResponseWriter, *stdhttp.Request) *T) Option[T] {
	return optionFunc[T](func(cfg *builderConfig[T]) {
		cfg.contextFactory = fn
	})
}

func WithConfig[T any](cfgModule *config.ConfigModule) Option[T] {
	return optionFunc[T](func(cfg *builderConfig[T]) {
		cfg.config = cfgModule
	})
}

func WithHMR[T any](hmr commands.HMRFunc) Option[T] {
	return optionFunc[T](func(cfg *builderConfig[T]) {
		cfg.hmr = hmr
	})
}

func WithHideStartBanner[T any](hide bool) Option[T] {
	return optionFunc[T](func(cfg *builderConfig[T]) {
		cfg.hideStartBanner = hide
	})
}

func New[T any](options ...Option[T]) *App[T] {
	executableName := filepath.Base(os.Args[0])
	builtConfig := resolveAppConfig(options...)

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
		context:         builtConfig.context,
		contextFactory:  builtConfig.contextFactory,
		config:          builtConfig.config,
		hmr:             builtConfig.hmr,
		hideStartBanner: builtConfig.hideStartBanner,
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

	app.rootCmd.SetErr(newErrWriter())
	app.rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return app
}

func resolveAppConfig[T any](options ...Option[T]) builderConfig[T] {
	cfg := builderConfig[T]{}

	for _, option := range options {
		if option != nil {
			option.apply(&cfg)
		}
	}

	if cfg.config == nil {
		cfg.config = &config.ConfigModule{}
	}

	return cfg
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
		if init, ok := any(e.App).(interface{ Init() error }); ok {
			return init.Init()
		}
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
		if reset, ok := any(e.App).(interface{ Reset() error }); ok {
			return reset.Reset()
		}
		return e.Next()
	})
}

func (a *App[T]) skipBootstrap() bool {
	if a.bootstrapped {
		return true
	}

	flags := []string{"-h", "--help", "-v", "--version"}

	cmd, _, err := a.rootCmd.Find(os.Args[1:])
	if err != nil {
		return true
	}

	for _, arg := range os.Args[1:] {
		if !slices.Contains(flags, arg) {
			continue
		}

		trimmed := strings.TrimLeft(arg, "-")
		if len(trimmed) > 1 && cmd.Flags().Lookup(trimmed) == nil {
			return true
		}
		if len(trimmed) == 1 && cmd.Flags().ShorthandLookup(trimmed) == nil {
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

// Context returns the app-scoped T provided via WithContext.
func (a *App[T]) Context() *T {
	return a.context
}

func (a *App[T]) Config() *config.ConfigModule {
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
				logger := slog.Default()
				if a.config != nil && a.config.Logger != nil {
					logger = a.config.Logger
				}
				logger.Error("graceful shutdown incomplete, some connections were forcibly closed", "error", err)
			}

			if te.IsRestart {
				time.AfterFunc(3*time.Second, func() { wg.Done() })
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
					logger := slog.Default()
					if a.config != nil && a.config.Logger != nil {
						logger = a.config.Logger
					}
					logger.Error("HTTP redirect listener failed", "addr", config.HttpAddr, "error", err)
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

func newErrWriter() *coloredWriter {
	return &coloredWriter{
		w: os.Stderr,
		c: color.New(color.FgRed),
	}
}

type coloredWriter struct {
	w io.Writer
	c *color.Color
}

func (colored *coloredWriter) Write(p []byte) (n int, err error) {
	colored.c.SetWriter(colored.w)
	defer colored.c.UnsetWriter(colored.w)
	return colored.c.Print(string(p))
}
