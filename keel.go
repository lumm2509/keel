package keel

import (
	"context"
	"errors"
	"io"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/lumm2509/keel/apis"
	"github.com/lumm2509/keel/commands"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/container"
	"github.com/lumm2509/keel/runtime/hook"
	transporthttp "github.com/lumm2509/keel/transport/http"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/acme/autocert"
)

var Version = "(untracked)"

type bindRoutesFunc[Cradle any] func(container.Container[Cradle]) (*transporthttp.Router[*transporthttp.RequestEvent[Cradle]], error)
type HMRFunc func(context.Context) error
type ServeConfig = apis.ServeConfig

type BootstrapEvent[C any] struct {
	hook.Event
	Container container.Container[C]
}

type TerminateEvent[C any] struct {
	hook.Event
	Container container.Container[C]
	IsRestart bool
}

type ServeEvent[C any] struct {
	hook.Event
	Container   container.Container[C]
	Router      *transporthttp.Router[*transporthttp.RequestEvent[C]]
	Server      *stdhttp.Server
	CertManager *autocert.Manager
	Listener    net.Listener
}

type Config[Cradle any] struct {
	Container       container.Container[Cradle]
	HMR             HMRFunc
	HideStartBanner bool
}

func (cfg Config[Cradle]) apply(b *builderConfig[Cradle]) {
	b.container = cfg.Container
	b.hmr = cfg.HMR
	b.hideStartBanner = cfg.HideStartBanner
}

type App[Cradle any] struct {
	container       container.Container[Cradle]
	config          *config.ConfigModule
	bindRoutes      bindRoutesFunc[Cradle]
	hmr             HMRFunc
	hideStartBanner bool
	router          *transporthttp.Router[*transporthttp.RequestEvent[Cradle]]
	usesFacade      bool
	onBootstrap     *hook.Hook[*BootstrapEvent[Cradle]]
	onServe         *hook.Hook[*ServeEvent[Cradle]]
	onTerminate     *hook.Hook[*TerminateEvent[Cradle]]

	rootCmd *cobra.Command
}

type Option[Cradle any] interface {
	apply(*builderConfig[Cradle])
}

type builderConfig[Cradle any] struct {
	container       container.Container[Cradle]
	config          *config.ConfigModule
	cradle          *Cradle
	bindRoutes      bindRoutesFunc[Cradle]
	hmr             HMRFunc
	hideStartBanner bool
}

type optionFunc[Cradle any] func(*builderConfig[Cradle])

func (fn optionFunc[Cradle]) apply(cfg *builderConfig[Cradle]) {
	fn(cfg)
}

func WithCradle[Cradle any](cradle Cradle) Option[Cradle] {
	return optionFunc[Cradle](func(cfg *builderConfig[Cradle]) {
		value := cradle
		cfg.cradle = &value
	})
}

func WithConfig[Cradle any](cfgModule *config.ConfigModule) Option[Cradle] {
	return optionFunc[Cradle](func(cfg *builderConfig[Cradle]) {
		cfg.config = cfgModule
	})
}

func WithContainer[Cradle any](ctr container.Container[Cradle]) Option[Cradle] {
	return optionFunc[Cradle](func(cfg *builderConfig[Cradle]) {
		cfg.container = ctr
	})
}

func WithHMR[Cradle any](hmr HMRFunc) Option[Cradle] {
	return optionFunc[Cradle](func(cfg *builderConfig[Cradle]) {
		cfg.hmr = hmr
	})
}

func WithHideStartBanner[Cradle any](hide bool) Option[Cradle] {
	return optionFunc[Cradle](func(cfg *builderConfig[Cradle]) {
		cfg.hideStartBanner = hide
	})
}

func New[Cradle any](options ...Option[Cradle]) *App[Cradle] {
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

	app := &App[Cradle]{
		container:       builtConfig.container,
		config:          builtConfig.config,
		bindRoutes:      builtConfig.bindRoutes,
		hmr:             builtConfig.hmr,
		hideStartBanner: builtConfig.hideStartBanner,
		rootCmd:         rootCmd,
	}

	requestEventPool := sync.Pool{
		New: func() any {
			return &transporthttp.RequestEvent[Cradle]{}
		},
	}

	app.router = transporthttp.NewRouter(func(w stdhttp.ResponseWriter, r *stdhttp.Request) (*transporthttp.RequestEvent[Cradle], transporthttp.EventCleanupFunc) {
		event := requestEventPool.Get().(*transporthttp.RequestEvent[Cradle])
		event.Reset(app.container, w, r)

		return event, func() {
			event.Release()
			requestEventPool.Put(event)
		}
	})

	app.rootCmd.SetErr(newErrWriter())
	app.rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return app
}

func resolveAppConfig[Cradle any](options ...Option[Cradle]) builderConfig[Cradle] {
	cfg := builderConfig[Cradle]{}

	for _, option := range options {
		if option != nil {
			option.apply(&cfg)
		}
	}

	cfg.config = normalizeConfigModule(cfg.config)

	if cfg.container == nil {
		cfg.container = container.LoadBasecontainer(cfg.config, cfg.cradle)
	}

	return cfg
}

func normalizeConfigModule(cfg *config.ConfigModule) *config.ConfigModule {
	if cfg == nil {
		cfg = &config.ConfigModule{}
	}

	if cfg.Projectconfig.DataDir == nil || *cfg.Projectconfig.DataDir == "" {
		dataDir := filepath.Join(defaultBaseDir(), "pb_data")
		cfg.Projectconfig.DataDir = &dataDir
	}

	if cfg.Projectconfig.EncryptionEnv == nil {
		encryptionEnv := ""
		cfg.Projectconfig.EncryptionEnv = &encryptionEnv
	}

	return cfg
}

func defaultBaseDir() string {
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return wd
	}

	return filepath.Dir(os.Args[0])
}

func (a *App[Cradle]) Start() error {
	if len(os.Args) == 1 && a.usesFacade {
		a.rootCmd.SetArgs([]string{"develop"})
	}

	a.rootCmd.AddCommand(a.newDevelopCommand())
	return a.Execute()
}

func (a *App[Cradle]) Run() error {
	return a.Start()
}

func (a *App[Cradle]) Execute() error {
	if !a.skipBootstrap() {
		if err := a.bootstrap(); err != nil {
			return err
		}
	}

	done := make(chan error, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch
		done <- a.terminate(false)
	}()

	go func() {
		if err := a.rootCmd.ExecuteContext(context.Background()); err != nil {
			done <- err
			return
		}
		done <- a.terminate(false)
	}()

	return <-done
}

func (a *App[Cradle]) bootstrap() error {
	a.container.Logger().Info("application bootstrap started", "event", "app_bootstrap_started")

	event := &BootstrapEvent[Cradle]{Container: a.container}

	err := a.OnBootstrap().Trigger(event, func(e *BootstrapEvent[Cradle]) error {
		return e.Container.InitResources()
	})
	if err == nil && !a.container.ResourcesReady() {
		a.container.Logger().Warn("OnBootstrap hook didn't fail but container resources are still not initialized - maybe missing e.Next()?")
	}
	if err == nil {
		a.container.Logger().Info("application bootstrap completed", "event", "app_bootstrap_completed")
	}

	return err
}

func (a *App[Cradle]) terminate(isRestart bool) error {
	event := &TerminateEvent[Cradle]{
		Container: a.container,
		IsRestart: isRestart,
	}

	err := a.OnTerminate().Trigger(event, func(e *TerminateEvent[Cradle]) error {
		return e.Container.ResetResources()
	})
	if err == nil {
		a.container.Logger().Info("application terminated", "event", "app_terminated", "restart", isRestart)
	}
	return err
}

func (a *App[Cradle]) skipBootstrap() bool {
	if a.container == nil || a.container.ResourcesReady() {
		return true
	}

	flags := []string{"-h", "--help", "-v", "--version"}

	cmd, _, err := a.rootCmd.Find(os.Args[1:])
	if err != nil {
		return true
	}

	for _, arg := range os.Args[1:] {
		if !contains(flags, arg) {
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

func (a *App[Cradle]) OnBootstrap() *hook.Hook[*BootstrapEvent[Cradle]] {
	if a.onBootstrap == nil {
		a.onBootstrap = &hook.Hook[*BootstrapEvent[Cradle]]{}
	}
	return a.onBootstrap
}

func (a *App[Cradle]) OnServe() *hook.Hook[*ServeEvent[Cradle]] {
	if a.onServe == nil {
		a.onServe = &hook.Hook[*ServeEvent[Cradle]]{}
	}
	return a.onServe
}

func (a *App[Cradle]) OnTerminate() *hook.Hook[*TerminateEvent[Cradle]] {
	if a.onTerminate == nil {
		a.onTerminate = &hook.Hook[*TerminateEvent[Cradle]]{}
	}
	return a.onTerminate
}

func (a *App[Cradle]) Container() container.Container[Cradle] {
	return a.container
}

func (a *App[Cradle]) Config() *config.ConfigModule {
	return a.config
}

func (a *App[Cradle]) Use(middlewares ...Middleware[Cradle]) *App[Cradle] {
	a.usesFacade = true
	for _, middleware := range middlewares {
		a.router.BindFunc(middleware)
	}
	return a
}

func (a *App[Cradle]) Get(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.GET(path, handler)
	return a
}

func (a *App[Cradle]) GET(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Get(path, handler)
}

func (a *App[Cradle]) Post(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.POST(path, handler)
	return a
}

func (a *App[Cradle]) POST(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Post(path, handler)
}

func (a *App[Cradle]) Put(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.PUT(path, handler)
	return a
}

func (a *App[Cradle]) PUT(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Put(path, handler)
}

func (a *App[Cradle]) Patch(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.PATCH(path, handler)
	return a
}

func (a *App[Cradle]) PATCH(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Patch(path, handler)
}

func (a *App[Cradle]) Delete(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.DELETE(path, handler)
	return a
}

func (a *App[Cradle]) DELETE(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Delete(path, handler)
}

func (a *App[Cradle]) Head(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.HEAD(path, handler)
	return a
}

func (a *App[Cradle]) HEAD(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Head(path, handler)
}

func (a *App[Cradle]) Options(path string, handler Handler[Cradle]) *App[Cradle] {
	a.usesFacade = true
	a.router.OPTIONS(path, handler)
	return a
}

func (a *App[Cradle]) OPTIONS(path string, handler Handler[Cradle]) *App[Cradle] {
	return a.Options(path, handler)
}

func (a *App[Cradle]) Group(prefix string, fn func(*Group[Cradle])) *Group[Cradle] {
	a.usesFacade = true
	group := &Group[Cradle]{inner: a.router.Group(prefix)}
	if fn != nil {
		fn(group)
	}
	return group
}

func (a *App[Cradle]) BuildMux() (stdhttp.Handler, error) {
	return a.router.BuildMux()
}

func (a *App[Cradle]) routeBinder() bindRoutesFunc[Cradle] {
	if a.bindRoutes != nil {
		return a.bindRoutes
	}

	return func(container.Container[Cradle]) (*transporthttp.Router[*transporthttp.RequestEvent[Cradle]], error) {
		return a.router, nil
	}
}

func (a *App[Cradle]) newDevelopCommand() *cobra.Command {
	var hmr commands.HMRFunc
	if a.hmr != nil {
		hmr = func(ctx context.Context) error {
			return a.hmr(ctx)
		}
	}

	return commands.NewDevelopCommand(
		hmr,
		!a.hideStartBanner,
		func(cfg apis.ServeConfig) error {
			if !a.usesFacade {
				return errors.New("develop command requires routes")
			}
			err := a.serve(cfg)
			if errors.Is(err, stdhttp.ErrServerClosed) {
				return nil
			}
			return err
		},
	)
}

func (a *App[Cradle]) serve(config ServeConfig) error {
	prepared, err := apis.PrepareServe(a.container, a.config, config, a.routeBinder())
	if err != nil {
		return err
	}
	defer prepared.Close()

	var listener net.Listener
	var wg sync.WaitGroup

	a.OnTerminate().Bind(&hook.Handler[*TerminateEvent[Cradle]]{
		Id: "keelGracefulShutdown",
		Func: func(te *TerminateEvent[Cradle]) error {
			prepared.Close()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			wg.Add(1)
			_ = prepared.Server.Shutdown(ctx)

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

	serveEvent := &ServeEvent[Cradle]{
		Container:   a.container,
		Router:      prepared.Router,
		Server:      prepared.Server,
		CertManager: prepared.CertManager,
		Listener:    prepared.Listener,
	}

	if err := a.OnServe().Trigger(serveEvent, func(e *ServeEvent[Cradle]) error {
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

	a.container.Logger().Info(
		"application server started",
		"event", "app_server_started",
		"addr", listener.Addr().String(),
		"base_url", baseURL,
		"https", config.HttpsAddr != "",
	)

	if config.HttpsAddr != "" {
		if config.HttpAddr != "" && prepared.CertManager != nil {
			go func() {
				_ = stdhttp.ListenAndServe(config.HttpAddr, prepared.CertManager.HTTPHandler(nil))
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

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
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
