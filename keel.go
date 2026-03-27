package keel

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	commands "github.com/lumm2509/keel/commands"
	"github.com/lumm2509/keel/container"
	transporthttp "github.com/lumm2509/keel/transport/http"
	"github.com/spf13/cobra"
)

var Version = "(untracked)"

type Config[Cradle any] struct {
	Container       container.Container[Cradle]
	BindRoutes      commands.BindRoutesFunc[Cradle]
	HMR             commands.HMRFunc
	HideStartBanner bool
}

type App[Cradle any] struct {
	container       container.Container[Cradle]
	bindRoutes      commands.BindRoutesFunc[Cradle]
	hmr             commands.HMRFunc
	hideStartBanner bool

	RootCmd *cobra.Command
}

func New[Cradle any](config Config[Cradle]) *App[Cradle] {
	executableName := filepath.Base(os.Args[0])

	app := &App[Cradle]{
		container:       config.Container,
		bindRoutes:      config.BindRoutes,
		hmr:             config.HMR,
		hideStartBanner: config.HideStartBanner,
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

func (a *App[Cradle]) Container() container.Container[Cradle] {
	return a.container
}

func (a *App[Cradle]) Start() error {
	a.RootCmd.AddCommand(commands.NewDevelopCommand(a.container, a.bindRoutes, a.hmr, !a.hideStartBanner))
	return a.Execute()
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

type BindRoutesFunc[Cradle any] func(container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error)
