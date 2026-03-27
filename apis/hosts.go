package apis

import (
	"net"
	"strings"
)

func HostNames(mainAddr string, certificateDomains []string) ([]string, []string) {
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
		var exists bool
		for _, item := range hostNames {
			if item == wwwHost {
				exists = true
				break
			}
		}
		if !exists {
			hostNames = append(hostNames, wwwHost)
			wwwRedirects = append(wwwRedirects, wwwHost)
		}
	}

	return hostNames, wwwRedirects
}
