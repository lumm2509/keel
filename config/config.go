package config

import "log/slog"

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
