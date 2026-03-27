package commands

import (
	"context"
	"errors"

	"github.com/lumm2509/keel/apis"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type HMRFunc func(ctx context.Context) error

func NewDevelopCommand(hmr HMRFunc, showStartBanner bool, serve func(apis.ServeConfig) error) *cobra.Command {
	var allowedOrigins []string
	var httpAddr string
	var httpsAddr string

	command := &cobra.Command{
		Use:          "develop [domain(s)]",
		Args:         cobra.ArbitraryArgs,
		Short:        "Starts the dev server and optional HMR loop",
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			if serve == nil {
				return errors.New("develop command requires a serve function")
			}

			if len(args) > 0 {
				if httpAddr == "" {
					httpAddr = "0.0.0.0:80"
				}
				if httpsAddr == "" {
					httpsAddr = "0.0.0.0:443"
				}
			} else if httpAddr == "" {
				httpAddr = "127.0.0.1:8090"
			}

			g, ctx := errgroup.WithContext(command.Context())

			if hmr != nil {
				g.Go(func() error {
					err := hmr(ctx)
					if err != nil && !errors.Is(err, context.Canceled) {
						return err
					}
					return nil
				})
			}

			g.Go(func() error {
				return serve(apis.ServeConfig{
					HttpAddr:           httpAddr,
					HttpsAddr:          httpsAddr,
					ShowStartBanner:    showStartBanner,
					AllowedOrigins:     allowedOrigins,
					CertificateDomains: args,
				})
			})

			return g.Wait()
		},
	}

	command.PersistentFlags().StringSliceVar(
		&allowedOrigins,
		"origins",
		[]string{"*"},
		"CORS allowed domain origins list",
	)

	command.PersistentFlags().StringVar(
		&httpAddr,
		"http",
		"",
		"TCP address to listen for the HTTP server",
	)

	command.PersistentFlags().StringVar(
		&httpsAddr,
		"https",
		"",
		"TCP address to listen for the HTTPS server",
	)

	return command
}
