package apis

type ServeConfig struct {
	ShowStartBanner    bool
	HttpAddr           string
	HttpsAddr          string
	CertificateDomains []string
}
