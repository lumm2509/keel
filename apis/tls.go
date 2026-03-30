package apis

import (
	"crypto/tls"
	"errors"
	"path/filepath"

	"github.com/lumm2509/keel/config"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func CertManager(cfg *config.Config, dataDir string, hostNames []string) (*autocert.Manager, error) {
	if cfg == nil || cfg.Http == nil || cfg.Http.AutoCert == nil {
		return nil, nil
	}

	autoCert := cfg.Http.AutoCert
	cacheDir := ""
	if autoCert.CacheDir != nil {
		cacheDir = *autoCert.CacheDir
	}

	var cache autocert.Cache
	if cacheDir != "" {
		if dataDir == "" {
			return nil, errors.New("autocert cache dir requires a data dir to be set")
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

func TLSConfig(server *tls.Config, certManager *autocert.Manager) *tls.Config {
	if certManager == nil {
		return server
	}
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: certManager.GetCertificate,
		NextProtos:     []string{acme.ALPNProto},
	}
}
