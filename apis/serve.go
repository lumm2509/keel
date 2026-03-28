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
	"golang.org/x/crypto/acme/autocert"
)

type PreparedServe[Cradle any] struct {
	Router      *transporthttp.Router[*transporthttp.RequestEvent[Cradle]]
	Server      *stdhttp.Server
	CertManager *autocert.Manager
	Listener    net.Listener
	BaseURL     string
	cancelBase  context.CancelFunc
}

func (p *PreparedServe[Cradle]) Close() {
	if p.cancelBase != nil {
		p.cancelBase()
	}
	if p.Listener != nil {
		_ = p.Listener.Close()
	}
}

// Serve starts a new app web server.
func Serve[Cradle any](
	ctr container.Container[Cradle],
	cfg *config.ConfigModule,
	config ServeConfig,
	bindRoutes func(ctr container.Container[Cradle]) (*transporthttp.Router[*transporthttp.RequestEvent[Cradle]], error),
) error {
	prepared, err := PrepareServe(ctr, cfg, config, bindRoutes)
	if err != nil {
		return err
	}
	defer prepared.Close()

	if config.ShowStartBanner {
		StartBanner(prepared.BaseURL)
	}

	if config.HttpsAddr != "" {
		if config.HttpAddr != "" && prepared.CertManager != nil {
			go func() {
				_ = stdhttp.ListenAndServe(config.HttpAddr, prepared.CertManager.HTTPHandler(nil))
			}()
		}

		err = prepared.Server.ServeTLS(prepared.Listener, "", "")
	} else {
		err = prepared.Server.Serve(prepared.Listener)
	}

	if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		return err
	}

	return nil
}

func PrepareServe[Cradle any](
	ctr container.Container[Cradle],
	cfg *config.ConfigModule,
	config ServeConfig,
	bindRoutes func(ctr container.Container[Cradle]) (*transporthttp.Router[*transporthttp.RequestEvent[Cradle]], error),
) (*PreparedServe[Cradle], error) {
	config.AllowedOrigins = AllowedOrigins(HTTP(cfg), config.AllowedOrigins)

	router, err := bindRoutes(ctr)
	if err != nil {
		return nil, err
	}

	mainAddr := config.HttpAddr
	if config.HttpsAddr != "" {
		mainAddr = config.HttpsAddr
	}

	hostNames, wwwRedirects := HostNames(mainAddr, config.CertificateDomains)
	if len(wwwRedirects) > 0 {
		router.Bind(WWWRedirect[Cradle](wwwRedirects))
	}

	dataDir := ""
	if provider, ok := any(ctr).(container.DataDirProvider); ok {
		dataDir = provider.DataDir()
	}
	certManager, err := CertManager(cfg, dataDir, hostNames)
	if err != nil {
		return nil, err
	}

	baseCtx, cancelBaseCtx := context.WithCancel(context.Background())

	server := &stdhttp.Server{
		WriteTimeout:      5 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		Addr:              mainAddr,
		BaseContext: func(net.Listener) context.Context {
			return baseCtx
		},
		ErrorLog: log.New(&ServerErrorLogWriter[Cradle]{Container: ctr}, "", 0),
	}

	server.TLSConfig = TLSConfig(server.TLSConfig, certManager)

	handler, err := router.BuildMux()
	if err != nil {
		cancelBaseCtx()
		return nil, err
	}

	server.Handler = CORS(handler, config.AllowedOrigins)

	addr := server.Addr
	if addr == "" {
		if config.HttpsAddr != "" {
			addr = ":https"
		} else {
			addr = ":http"
		}
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		cancelBaseCtx()
		return nil, err
	}

	return &PreparedServe[Cradle]{
		Router:      router,
		Server:      server,
		CertManager: certManager,
		Listener:    listener,
		BaseURL:     BaseURL(config, server.Addr),
		cancelBase:  cancelBaseCtx,
	}, nil
}

type ServerErrorLogWriter[Cradle any] struct {
	Container container.Container[Cradle]
}

func (s *ServerErrorLogWriter[Cradle]) Write(p []byte) (int, error) {
	s.Container.Logger().Debug(strings.TrimSpace(string(p)))
	return len(p), nil
}

func WWWRedirect[Cradle any](hosts []string) *hook.Handler[*transporthttp.RequestEvent[Cradle]] {
	return &hook.Handler[*transporthttp.RequestEvent[Cradle]]{
		Id: "wwwRedirect",
		Func: func(e *transporthttp.RequestEvent[Cradle]) error {
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
