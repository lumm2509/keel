package config

import "log/slog"

// ResolveLogger returns cfg.Logger when set, otherwise slog.Default().
// It is nil-safe: calling it on a nil *Config returns slog.Default().
func (cfg *Config) ResolveLogger() *slog.Logger {
	if cfg != nil && cfg.Logger != nil {
		return cfg.Logger
	}
	return slog.Default()
}

type Config struct {
	Logger  *slog.Logger `json:"-"`
	DataDir *string      `json:"dataDir,omitempty"`
	Http    *HttpConfig  `json:"http,omitempty"`
}

type HttpConfig struct {
	AutoCert *AutoCertConfig `json:"autoCert,omitempty"`
}

type AutoCertConfig struct {
	CacheDir      *string  `json:"cacheDir,omitempty"`
	HostWhitelist []string `json:"hostWhitelist,omitempty"`
	Email         *string  `json:"email,omitempty"`
}
