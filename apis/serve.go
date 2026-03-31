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
	transporthttp "github.com/lumm2509/keel/transport/http"
	"golang.org/x/crypto/acme/autocert"
)


type dataDirProvider interface {
	DataDir() string
}

type PreparedServe[T any] struct {
	Router      *transporthttp.Router[*transporthttp.RequestEvent[T]]
	Server      *stdhttp.Server
	CertManager *autocert.Manager
	Listener    net.Listener
	BaseURL     string
	cancelBase  context.CancelFunc
}


func (p *PreparedServe[T]) Close() {
	if p.cancelBase != nil {
		p.cancelBase()
	}
	if p.Listener != nil {
		_ = p.Listener.Close()
	}
}

// Serve starts a new app web server.
func Serve[T any](
	ctx *T,
	cfg *config.Config,
	config ServeConfig,
	router *transporthttp.Router[*transporthttp.RequestEvent[T]],
) error {
	prepared, err := PrepareServe(ctx, cfg, config, router)
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
				if err := stdhttp.ListenAndServe(config.HttpAddr, prepared.CertManager.HTTPHandler(nil)); err != nil {
					cfg.ResolveLogger().Error("HTTP redirect listener failed", "addr", config.HttpAddr, "error", err)
				}
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

func PrepareServe[T any](
	ctx *T,
	cfg *config.Config,
	config ServeConfig,
	router *transporthttp.Router[*transporthttp.RequestEvent[T]],
) (*PreparedServe[T], error) {
	mainAddr := config.HttpAddr
	if config.HttpsAddr != "" {
		mainAddr = config.HttpsAddr
	}

	hostNames, _ := HostNames(mainAddr, config.CertificateDomains)

	dataDir := ""
	if provider, ok := any(ctx).(dataDirProvider); ok {
		dataDir = provider.DataDir()
	} else if cfg != nil && cfg.DataDir != nil {
		dataDir = *cfg.DataDir
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
		ErrorLog: log.New(&serverErrorLogWriter{cfg}, "", 0),
	}

	server.TLSConfig = TLSConfig(server.TLSConfig, certManager)

	if _, err := router.BuildMux(); err != nil {
		cancelBaseCtx()
		return nil, err
	}

	server.Handler = router.Handler()

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

	return &PreparedServe[T]{
		Router:      router,
		Server:      server,
		CertManager: certManager,
		Listener:    listener,
		BaseURL:     BaseURL(config, server.Addr),
		cancelBase:  cancelBaseCtx,
	}, nil
}

type serverErrorLogWriter struct {
	config *config.Config
}

func (s *serverErrorLogWriter) Write(p []byte) (int, error) {
	s.config.ResolveLogger().Debug(strings.TrimSpace(string(p)))
	return len(p), nil
}

