package apis

import "strings"

func BaseURL(config ServeConfig, addr string) string {
	host := host(addr)
	if config.HttpsAddr != "" {
		if len(config.CertificateDomains) > 0 {
			host = config.CertificateDomains[0]
		}
		return "https://" + host
	}
	return "http://" + host
}

func host(addr string) string {
	if addr == "" || strings.HasSuffix(addr, ":http") || strings.HasSuffix(addr, ":https") {
		return "127.0.0.1"
	}
	return addr
}
