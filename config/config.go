package config

import "log/slog"

// ConfigModule is a runtime configuration aggregate.
// It is a convenience boundary for app settings, not an architectural layer by itself.
type ConfigModule struct {
	Projectconfig ProjectConfigOptions `json:"projectConfig,omitempty"`
	Logger        *slog.Logger         `json:"-"`
}

// ProjectConfigOptions holds the subset of settings used by the keel v0.0.1 core:
// database connection, HTTP options (CORS, TLS), and dev mode flag.
type ProjectConfigOptions struct {
	DatabaseName          *string                `json:"databaseName,omitempty"`
	DatabaseUrl           *string                `json:"databaseUrl,omitempty"`
	DatabaseSchema        *string                `json:"databaseSchema,omitempty"`
	Databaselogging       *bool                  `json:"databaseLogging,omitempty"`
	DatabaseDriverOptions *DatabaseDriverOptions `json:"databaseDriverOptions,omitempty"`
	DataDir               *string                `json:"dataDir,omitempty"`
	EncryptionEnv         *string                `json:"encryptionEnv,omitempty"`
	IsDev                 bool                   `json:"isDev"`
	Http                  *HttpConfigOptions     `json:"http,omitempty"`
}

type DatabaseDriverOptions struct {
	PoolMin           *int `json:"poolMin,omitempty"`
	PoolMax           *int `json:"poolMax,omitempty"`
	IdleTimeoutMillis *int `json:"idleTimeoutMillis,omitempty"`
	ConnMaxLifetimeMs *int `json:"connMaxLifetimeMs,omitempty"`
	MaxRetries        *int `json:"maxRetries,omitempty"`
	RetryDelayMs      *int `json:"retryDelayMs,omitempty"`
}

// HttpConfigOptions holds HTTP-layer settings used by the core serve path.
type HttpConfigOptions struct {
	AllowedOrigins    []string `json:"allowedOrigins,omitempty"`
	TrustedProxyCIDRs []string `json:"trustedProxyCidrs,omitempty"`
	AutoCert          *struct {
		CacheDir      *string  `json:"cacheDir,omitempty"`
		HostWhitelist []string `json:"hostWhitelist,omitempty"`
		Email         *string  `json:"email,omitempty"`
	} `json:"autoCert,omitempty"`
}
