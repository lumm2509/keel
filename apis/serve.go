package apis

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	stdhttp "net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/lumm2509/keel/container"
	"github.com/lumm2509/keel/pkg/list"
	"github.com/lumm2509/keel/runtime/hook"
	transporthttp "github.com/lumm2509/keel/transport/http"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// ServeConfig defines a configuration struct for apis.Serve().
type ServeConfig struct {
	ShowStartBanner    bool
	HttpAddr           string
	HttpsAddr          string
	CertificateDomains []string
	AllowedOrigins     []string
}

// Serve starts a new app web server.
func Serve[Cradle any](
	ctr container.Container[Cradle],
	config ServeConfig,
	bindRoutes func(ctr container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error),
) error {
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}

	router, err := bindRoutes(ctr)
	if err != nil {
		return err
	}

	mainAddr := config.HttpAddr
	if config.HttpsAddr != "" {
		mainAddr = config.HttpsAddr
	}

	hostNames, wwwRedirects := collectHostNames(mainAddr, config.CertificateDomains)
	if len(wwwRedirects) > 0 {
		router.Bind(wwwRedirect[Cradle](wwwRedirects))
	}

	certManager, err := buildCertManager(ctr, hostNames)
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

	if certManager != nil {
		server.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: certManager.GetCertificate,
			NextProtos:     []string{acme.ALPNProto},
		}
	}

	var listener net.Listener
	var wg sync.WaitGroup

	ctr.OnTerminate().Bind(&hook.Handler[*container.TerminateEvent[Cradle]]{
		Id: "keelGracefulShutdown",
		Func: func(te *container.TerminateEvent[Cradle]) error {
			cancelBaseCtx()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			wg.Add(1)
			_ = server.Shutdown(ctx)

			if te.IsRestart {
				time.AfterFunc(3*time.Second, func() {
					wg.Done()
				})
			} else {
				wg.Done()
			}

			return te.Next()
		},
		Priority: -9999,
	})

	defer func() {
		wg.Wait()
		if listener != nil {
			_ = listener.Close()
		}
	}()

	var baseURL string

	serveEvent := &container.ServeEvent[Cradle]{
		Container:   ctr,
		Router:      router,
		Server:      server,
		CertManager: certManager,
	}

	if err := ctr.OnServe().Trigger(serveEvent, func(e *container.ServeEvent[Cradle]) error {
		handler, err := e.Router.BuildMux()
		if err != nil {
			return err
		}

		e.Server.Handler = handler
		baseURL = resolveBaseURL(config, e.Server.Addr)

		addr := e.Server.Addr
		if addr == "" {
			if config.HttpsAddr != "" {
				addr = ":https"
			} else {
				addr = ":http"
			}
		}

		if e.Listener == nil {
			listener, err = net.Listen("tcp", addr)
			if err != nil {
				return err
			}
		} else {
			listener = e.Listener
		}

		return nil
	}); err != nil {
		return err
	}

	if listener == nil {
		return errors.New("the OnServe listener was not initialized; did you forget to call e.Next()?")
	}

	if config.ShowStartBanner {
		printStartBanner(baseURL)
	}

	if config.HttpsAddr != "" {
		if config.HttpAddr != "" && certManager != nil {
			go func() {
				_ = stdhttp.ListenAndServe(config.HttpAddr, certManager.HTTPHandler(nil))
			}()
		}

		err = serveEvent.Server.ServeTLS(listener, "", "")
	} else {
		err = serveEvent.Server.Serve(listener)
	}

	if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
		return err
	}

	return nil
}

func collectHostNames(mainAddr string, certificateDomains []string) ([]string, []string) {
	var wwwRedirects []string
	hostNames := append([]string(nil), certificateDomains...)

	if len(hostNames) == 0 {
		host, _, _ := net.SplitHostPort(mainAddr)
		if host != "" {
			hostNames = append(hostNames, host)
		}
	}

	for _, host := range append([]string(nil), hostNames...) {
		if host == "" || strings.HasPrefix(host, "www.") {
			continue
		}

		wwwHost := "www." + host
		if !list.ExistInSlice(wwwHost, hostNames) {
			hostNames = append(hostNames, wwwHost)
			wwwRedirects = append(wwwRedirects, wwwHost)
		}
	}

	return hostNames, wwwRedirects
}

func buildCertManager[Cradle any](ctr container.Container[Cradle], hostNames []string) (*autocert.Manager, error) {
	cfg := ctr.Config()
	if cfg == nil {
		return nil, nil
	}

	httpConfig := cfg.Projectconfig.Http
	if httpConfig == nil || httpConfig.AutoCert == nil {
		return nil, nil
	}

	autoCert := httpConfig.AutoCert
	cacheDir := ""
	if autoCert.CacheDir != nil {
		cacheDir = *autoCert.CacheDir
	}

	var cache autocert.Cache
	if cacheDir != "" {
		dataDir := ctr.DataDir()
		if dataDir == "" {
			return nil, errors.New("autocert cache dir requires container data dir")
		}

		cache = autocert.DirCache(filepath.Join(dataDir, cacheDir))
	}

	hosts := hostNames
	if len(autoCert.HostWhitelist) > 0 {
		hosts = autoCert.HostWhitelist
	}

	manager := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  cache,
	}

	if autoCert.Email != nil {
		manager.Email = *autoCert.Email
	}

	if len(hosts) > 0 {
		manager.HostPolicy = autocert.HostWhitelist(hosts...)
	}

	return manager, nil
}

func resolveBaseURL(config ServeConfig, addr string) string {
	host := serverAddrToHost(addr)
	if config.HttpsAddr != "" {
		if len(config.CertificateDomains) > 0 {
			host = config.CertificateDomains[0]
		}
		return "https://" + host
	}

	return "http://" + host
}

func printStartBanner(baseURL string) {
	date := new(strings.Builder)
	log.New(date, "", log.LstdFlags).Print()

	bold := color.New(color.Bold).Add(color.FgGreen)
	bold.Printf("%s Server started at %s\n", strings.TrimSpace(date.String()), color.CyanString("%s", baseURL))

	regular := color.New()
	regular.Printf("├─ REST API:  %s\n", color.CyanString("%s/api/", baseURL))
	regular.Printf("└─ Dashboard: %s\n", color.CyanString("%s/_/", baseURL))
}

func serverAddrToHost(addr string) string {
	if addr == "" || strings.HasSuffix(addr, ":http") || strings.HasSuffix(addr, ":https") {
		return "127.0.0.1"
	}
	return addr
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
