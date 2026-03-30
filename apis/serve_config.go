package apis

import "time"

type ServeConfig struct {
	ShowStartBanner    bool
	HttpAddr           string
	HttpsAddr          string
	CertificateDomains []string
	// ShutdownTimeout is the maximum time to wait for active connections to
	// finish during graceful shutdown. Defaults to 30 seconds if zero or negative.
	ShutdownTimeout time.Duration
}
