package apis

import (
	"context"
	"errors"
	"log"
	"net"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/container"
	"github.com/lumm2509/keel/pkg/list"
	"github.com/lumm2509/keel/runtime/hook"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

// Serve starts a new app web server.
func Serve[Cradle any](
	ctr container.Container[Cradle],
	cfg *config.ConfigModule,
	config ServeConfig,
	bindRoutes func(ctr container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error),
) error {
	config.AllowedOrigins = AllowedOrigins(HTTP(cfg), config.AllowedOrigins)

	router, err := bindRoutes(ctr)
	if err != nil {
		return err
	}

	mainAddr := config.HttpAddr
	if config.HttpsAddr != "" {
		mainAddr = config.HttpsAddr
	}

	hostNames, wwwRedirects := HostNames(mainAddr, config.CertificateDomains)
	if len(wwwRedirects) > 0 {
		router.Bind(wwwRedirect[Cradle](wwwRedirects))
	}

	dataDir := ""
	if provider, ok := any(ctr).(container.DataDirProvider); ok {
		dataDir = provider.DataDir()
	}
	certManager, err := CertManager(cfg, dataDir, hostNames)
	if err != nil {
		return err
	}

	baseCtx, cancelBaseCtx := context.WithCancel(context.Background())
	defer cancelBaseCtx()

	server := &stdhttp.Server{
		WriteTimeout:      5 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		Addr:              mainAddr,
		BaseContext: func(net.Listener) context.Context {
			return baseCtx
		},
		ErrorLog: log.New(&serverErrorLogWriter[Cradle]{container: ctr}, "", 0),
	}

	server.TLSConfig = TLSConfig(server.TLSConfig, certManager)

	var listener net.Listener

	defer func() {
		cancelBaseCtx()
		if listener != nil {
			_ = listener.Close()
		}
	}()

	var baseURL string

	handler, err := router.BuildMux()
	if err != nil {
		return err
	}

	server.Handler = CORS(handler, config.AllowedOrigins)
	baseURL = BaseURL(config, server.Addr)

	addr := server.Addr
	if addr == "" {
		if config.HttpsAddr != "" {
			addr = ":https"
		} else {
			addr = ":http"
		}
	}

	listener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	if config.ShowStartBanner {
		StartBanner(baseURL)
	}

	if config.HttpsAddr != "" {
		if config.HttpAddr != "" && certManager != nil {
			go func() {
				_ = stdhttp.ListenAndServe(config.HttpAddr, certManager.HTTPHandler(nil))
			}()
		}

		err = server.ServeTLS(listener, "", "")
	} else {
		err = server.Serve(listener)
	}

	if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		return err
	}

	return nil
}

type serverErrorLogWriter[Cradle any] struct {
	container container.Container[Cradle]
}

func (s *serverErrorLogWriter[Cradle]) Write(p []byte) (int, error) {
	s.container.Logger().Debug(strings.TrimSpace(string(p)))
	return len(p), nil
}

func wwwRedirect[Cradle any](hosts []string) *hook.Handler[*container.RequestEvent[Cradle]] {
	return &hook.Handler[*container.RequestEvent[Cradle]]{
		Id: "wwwRedirect",
		Func: func(e *container.RequestEvent[Cradle]) error {
			if e.Request == nil || e.Request.URL == nil {
				return e.Next()
			}

			host := e.Request.Host
			if host == "" {
				host = e.Request.URL.Host
			}

			if !list.ExistInSlice(host, hosts) {
				return e.Next()
			}

			targetHost := strings.TrimPrefix(host, "www.")
			target := *e.Request.URL
			target.Host = targetHost
			target.Scheme = "https"

			if target.Path == "" {
				target.Path = "/"
			}

			stdhttp.Redirect(e.Response, e.Request, target.String(), stdhttp.StatusPermanentRedirect)
			return nil
		},
		Priority: -9999,
	}
}
