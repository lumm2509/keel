package config

import "log/slog"

type ModuleOptions interface{}

type AdminOptions struct {
	Disable           *bool   `json:"disable,omitempty" xml:"disable,omitempty" form:"disable,omitempty"`
	Path              *string `json:"path,omitempty" xml:"path,omitempty" form:"path,omitempty"`
	BackendUrl        *string `json:"backendUrl,omitempty" xml:"backendUrl,omitempty" form:"backendUrl,omitempty"`
	OutDir            *string `json:"outDir,omitempty" xml:"outDir,omitempty" form:"outDir,omitempty"`
	MaxUploadfileSize *int32  `json:"maxUploadfileSize,omitempty" xml:"maxUploadfileSize,omitempty" form:"maxUploadfileSize,omitempty"`
}

type SessionOptions struct {
	Name              *string `json:"name,omitempty" xml:"name,omitempty" form:"name,omitempty"`
	Resave            *bool   `json:"resave,omitempty" xml:"resave,omitempty" form:"resave,omitempty"`
	Rooling           *bool   `json:"rooling,omitempty" xml:"rooling,omitempty" form:"rooling,omitempty"`
	SaveUninitialized *bool   `json:"saveUninitialized,omitempty" xml:"saveUninitialized,omitempty" form:"saveUninitialized,omitempty"`
	Secret            *string `json:"secret,omitempty" xml:"secret,omitempty" form:"secret,omitempty"`
	Ttl               *int32  `json:"ttl,omitempty" xml:"ttl,omitempty" form:"ttl,omitempty"`
}

type CookieOptionsSameSite string

const (
	Lax    CookieOptionsSameSite = "lax"
	Strict CookieOptionsSameSite = "strict"
	None   CookieOptionsSameSite = "none"
)

type CookieOptionsPriority string

const (
	Low    CookieOptionsPriority = "low"
	Medium CookieOptionsPriority = "medium"
	High   CookieOptionsPriority = "high"
)

type CookieOptions struct {
	Secure   *bool                  `json:"secure,omitempty" xml:"secure,omitempty" form:"secure,omitempty"`
	SameSite *CookieOptionsSameSite `json:"sameSite,omitempty" xml:"sameSite,omitempty" form:"sameSite,omitempty"`
	MaxAge   *int32                 `json:"maxAge,omitempty" xml:"maxAge,omitempty" form:"maxAge,omitempty"`
	HttpOnly *bool                  `json:"httpOnly,omitempty" xml:"httpOnly,omitempty" form:"httpOnly,omitempty"`
	Priority *CookieOptionsPriority `json:"priority,omitempty" xml:"priority,omitempty" form:"priority,omitempty"`
	Domain   *string                `json:"domain,omitempty" xml:"domain,omitempty" form:"domain,omitempty"`
	Path     *string                `json:"path,omitempty" xml:"path,omitempty" form:"path,omitempty"`
	Signed   *bool                  `json:"signed,omitempty" xml:"signed,omitempty" form:"signed,omitempty"`
}

type HttpCompressionOptions struct {
	Enabled   *bool  `json:"enabled,omitempty" xml:"enabled,omitempty" form:"enabled,omitempty"`
	Level     *int32 `json:"level,omitempty" xml:"level,omitempty" form:"level,omitempty"`
	MemLevel  *int32 `json:"memLevel,omitempty" xml:"memLevel,omitempty" form:"memLevel,omitempty"`
	ThresHold *int32 `json:"thresHold,omitempty" xml:"thresHold,omitempty" form:"thresHold,omitempty"`
}

type JWTOptions struct {
	Secret *string `json:"secret,omitempty" xml:"secret,omitempty" form:"secret,omitempty"`
}

type HttpConfigOptions struct {
	JWT               *JWTOptions             `json:"jwt,omitempty" xml:"jwt,omitempty" form:"jwt,omitempty"`
	CookieSecret      *string                 `json:"cookieSecret,omitempty" xml:"cookieSecret,omitempty" form:"cookieSecret,omitempty"`
	AllowedOrigins    []string                `json:"allowedOrigins,omitempty" xml:"allowedOrigins,omitempty" form:"allowedOrigins,omitempty"`
	TrustedProxyCIDRs []string                `json:"trustedProxyCidrs,omitempty" xml:"trustedProxyCidrs,omitempty" form:"trustedProxyCidrs,omitempty"`
	Compression       *HttpCompressionOptions `json:"compression,omitempty" xml:"compression,omitempty" form:"compression,omitempty"`
	AutoCert          *struct {
		CacheDir      *string  `json:"cacheDir,omitempty"`
		HostWhitelist []string `json:"hostWhitelist,omitempty"`
		Email         *string  `json:"email,omitempty"`
	} `json:"autoCert,omitempty"`
}

type KeelCloudOptions struct {
	EnviromentHandle *string `json:"enviromentHandle,omitempty" xml:"enviromentHandle,omitempty" form:"enviromentHandle,omitempty"`
	SandboxHandle    *string `json:"sandboxHandle,omitempty" xml:"sandboxHandle,omitempty" form:"sandboxHandle,omitempty"`
	ApiKey           *string `json:"apiKey,omitempty" xml:"apiKey,omitempty" form:"apiKey,omitempty"`
	WebhookSecret    *string `json:"webhookSecret,omitempty" xml:"webhookSecret,omitempty" form:"webhookSecret,omitempty"`
}

type ProjectConfigOptionsWorkerMode string

const (
	Shared ProjectConfigOptionsWorkerMode = "shared"
	Worker ProjectConfigOptionsWorkerMode = "worker"
	Server ProjectConfigOptionsWorkerMode = "server"
)

type DatabaseDriverOptions struct {
	PoolMin           *int `json:"poolMin,omitempty" xml:"poolMin,omitempty" form:"poolMin,omitempty"`
	PoolMax           *int `json:"poolMax,omitempty" xml:"poolMax,omitempty" form:"poolMax,omitempty"`
	IdleTimeoutMillis *int `json:"idleTimeoutMillis,omitempty" xml:"idleTimeoutMillis,omitempty" form:"idleTimeoutMillis,omitempty"`
	ConnMaxLifetimeMs *int `json:"connMaxLifetimeMs,omitempty" xml:"connMaxLifetimeMs,omitempty" form:"connMaxLifetimeMs,omitempty"`
	MaxRetries        *int `json:"maxRetries,omitempty" xml:"maxRetries,omitempty" form:"maxRetries,omitempty"`
	RetryDelayMs      *int `json:"retryDelayMs,omitempty" xml:"retryDelayMs,omitempty" form:"retryDelayMs,omitempty"`
}

type ProjectConfigOptions struct {
	DatabaseName          *string                `json:"databaseName,omitempty" xml:"databaseName,omitempty" form:"databaseName,omitempty"`
	DatabaseUrl           *string                `json:"databaseUrl,omitempty" xml:"databaseUrl,omitempty" form:"databaseUrl,omitempty"`
	DatabaseSchema        *string                `json:"databaseSchema,omitempty" xml:"databaseSchema,omitempty" form:"databaseSchema,omitempty"`
	Databaselogging       *bool                  `json:"databaseLogging,omitempty" xml:"databaseLogging,omitempty" form:"databaseLogging,omitempty"`
	DatabaseDriverOptions *DatabaseDriverOptions `json:"databaseDriverOptions,omitempty" xml:"databaseDriverOptions,omitempty" form:"databaseDriverOptions,omitempty"`
	RedisUrl              *string                `json:"redisUrl,omitempty" xml:"redisUrl,omitempty" form:"redisUrl,omitempty"`
	RedisPrefix           *string                `json:"redisPrefix,omitempty" xml:"redisPrefix,omitempty" form:"redisPrefix,omitempty"`
	SessionOptions        *SessionOptions        `json:"sessionOptions,omitempty" xml:"sessionOptions,omitempty" form:"sessionOptions,omitempty"`
	CookieOptions         *CookieOptions         `json:"cookieOptions,omitempty" xml:"cookieOptions,omitempty" form:"cookieOptions,omitempty"`
	JobsBatchSize         *int32                 `json:"jobsBatchSize,omitempty" xml:"jobsBatchSize,omitempty" form:"jobsBatchSize,omitempty"`
	IsDev                 bool                   `json:"isDev" xml:"isDev" form:"isDev"`
	Http                  *HttpConfigOptions     `json:"http,omitempty" xml:"http,omitempty" form:"http,omitempty"`
	Cloud                 *KeelCloudOptions      `json:"cloud,omitempty" xml:"cloud,omitempty" form:"cloud,omitempty"`
}

// ConfigModule is a runtime configuration aggregate.
// It is a convenience boundary for app settings, not an architectural layer by itself.
type ConfigModule struct {
	Projectconfig ProjectConfigOptions `json:"projectConfig,omitempty" xml:"projectConfig,omitempty" form:"projectConfig,omitempty"`
	Admin         AdminOptions         `json:"admin,omitempty" xml:"admin,omitempty" form:"admin,omitempty"`
	FeatureFlags  map[string]any       `json:"featureFlags,omitempty" xml:"featureFlags,omitempty" form:"featureFlags,omitempty"`
	Logger        *slog.Logger         `json:"-" xml:"-" form:"-"`
}

type PluginAdminDetailsType string

const (
	Local   PluginAdminDetailsType = "local"
	Package PluginAdminDetailsType = "package"
)

type PluginAdminDetails struct {
	Type    PluginAdminDetailsType `json:"type,omitempty" xml:"type,omitempty" form:"type,omitempty"`
	Resolve string                 `json:"resolve,omitempty" xml:"resolve,omitempty" form:"resolve,omitempty"`
}

type PluginDetails struct {
	Resolve string              `json:"resolve,omitempty" xml:"resolve,omitempty" form:"resolve,omitempty"`
	Name    string              `json:"name,omitempty" xml:"name,omitempty" form:"name,omitempty"`
	Id      string              `json:"id,omitempty" xml:"id,omitempty" form:"id,omitempty"`
	Options map[string]any      `json:"options,omitempty" xml:"options,omitempty" form:"options,omitempty"`
	Version string              `json:"version,omitempty" xml:"version,omitempty" form:"version,omitempty"`
	Admin   *PluginAdminDetails `json:"admin,omitempty" xml:"admin,omitempty" form:"admin,omitempty"`
}
