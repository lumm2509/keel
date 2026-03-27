package keel

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	commands "github.com/lumm2509/keel/commands"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/container"
	"github.com/lumm2509/keel/infra/filesystem"
	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/subscriptions"
	"github.com/lumm2509/keel/runtime/cron"
	"github.com/lumm2509/keel/runtime/hook"
	"github.com/spf13/cobra"
)

var Version = "(untracked)"

type Config[Cradle any] struct {
	Container       container.Container[Cradle]
	BindRoutes      commands.BindRoutesFunc[Cradle]
	HMR             commands.HMRFunc
	HideStartBanner bool
}

func (cfg Config[Cradle]) apply(b *builderConfig[Cradle]) {
	b.container = cfg.Container
	b.bindRoutes = cfg.BindRoutes
	b.hmr = cfg.HMR
	b.hideStartBanner = cfg.HideStartBanner
}

type App[Cradle any] struct {
	container       container.Container[Cradle]
	bindRoutes      commands.BindRoutesFunc[Cradle]
	hmr             commands.HMRFunc
	hideStartBanner bool
	routes          []routeRegistration[Cradle]

	RootCmd *cobra.Command
}

type Option[Cradle any] interface {
	apply(*builderConfig[Cradle])
}

type builderConfig[Cradle any] struct {
	container       container.Container[Cradle]
	config          *config.ConfigModule
	cradle          *Cradle
	bindRoutes      commands.BindRoutesFunc[Cradle]
	hmr             commands.HMRFunc
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

func WithHMR[Cradle any](hmr commands.HMRFunc) Option[Cradle] {
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

	app := &App[Cradle]{
		container:       builtConfig.container,
		bindRoutes:      builtConfig.bindRoutes,
		hmr:             builtConfig.hmr,
		hideStartBanner: builtConfig.hideStartBanner,
		RootCmd: &cobra.Command{
			Use:     executableName,
			Short:   executableName + " CLI",
			Version: Version,
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: true,
			},
			SilenceUsage: true,
		},
	}

	app.RootCmd.SetErr(newErrWriter())
	app.RootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return app
}

func resolveAppConfig[Cradle any](options ...Option[Cradle]) builderConfig[Cradle] {
	cfg := builderConfig[Cradle]{}

	for _, option := range options {
		if option == nil {
			continue
		}

		option.apply(&cfg)
	}

	if cfg.container == nil {
		cfg.container = container.LoadBasecontainer(cfg.config, cfg.cradle)
	}

	return cfg
}

// Container exposes the lower-level container for advanced composition paths.
func (a *App[Cradle]) Container() container.Container[Cradle] {
	return a.container
}

func (a *App[Cradle]) Cradle() *Cradle {
	return a.container.Cradle()
}

func (a *App[Cradle]) Config() *config.ConfigModule {
	return a.container.Config()
}

func (a *App[Cradle]) Logger() *slog.Logger {
	return a.container.Logger()
}

func (a *App[Cradle]) IsDev() bool {
	return a.container.IsDev()
}

func (a *App[Cradle]) DataBase() *sql.DB {
	return a.container.DataBase()
}

func (a *App[Cradle]) Cron() *cron.Cron {
	return a.container.Cron()
}

func (a *App[Cradle]) Store() *store.Store[string, any] {
	return a.container.Store()
}

func (a *App[Cradle]) SubscriptionsBroker() *subscriptions.Broker {
	return a.container.SubscriptionsBroker()
}

func (a *App[Cradle]) NewFilesystem() (*filesystem.System, error) {
	return a.container.NewFilesystem()
}

func (a *App[Cradle]) OnBootstrap() *hook.Hook[*container.BootstrapEvent[Cradle]] {
	return a.container.OnBootstrap()
}

func (a *App[Cradle]) OnServe() *hook.Hook[*container.ServeEvent[Cradle]] {
	return a.container.OnServe()
}

func (a *App[Cradle]) OnTerminate() *hook.Hook[*container.TerminateEvent[Cradle]] {
	return a.container.OnTerminate()
}

func (a *App[Cradle]) Start() error {
	if len(a.routes) > 0 {
		a.bindRoutes = a.composeBindRoutes()
	}

	if len(os.Args) == 1 && a.bindRoutes != nil {
		a.RootCmd.SetArgs([]string{"develop"})
	}

	a.RootCmd.AddCommand(commands.NewDevelopCommand(a.container, a.bindRoutes, a.hmr, !a.hideStartBanner))
	return a.Execute()
}

func (a *App[Cradle]) Run() error {
	return a.Start()
}

func (a *App[Cradle]) Execute() error {
	if !a.skipBootstrap() {
		if err := a.container.Bootstrap(); err != nil {
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
		err := a.RootCmd.ExecuteContext(context.Background())
		if err != nil {
			done <- err
			return
		}

		done <- a.terminate(false)
	}()

	return <-done
}

func (a *App[Cradle]) terminate(isRestart bool) error {
	event := &container.TerminateEvent[Cradle]{
		Container: a.container,
		IsRestart: isRestart,
	}

	return a.container.OnTerminate().Trigger(event, func(e *container.TerminateEvent[Cradle]) error {
		return e.Container.ResetBootstrapState()
	})
}

func (a *App[Cradle]) skipBootstrap() bool {
	if a.container == nil || a.container.IsBootstrapped() {
		return true
	}

	flags := []string{"-h", "--help", "-v", "--version"}

	cmd, _, err := a.RootCmd.Find(os.Args[1:])
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
