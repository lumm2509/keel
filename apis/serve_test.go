package apis

import (
	"testing"
	"time"

	"github.com/lumm2509/keel/config"
)

func TestBuildCertManagerWithoutConfigReturnsNil(t *testing.T) {
	t.Parallel()

	manager, err := buildCertManager(nil, "", nil)
	if err != nil {
		t.Fatalf("buildCertManager() error = %v", err)
	}

	if manager != nil {
		t.Fatalf("expected nil cert manager, got %#v", manager)
	}
}

func TestBuildCertManagerFailsWhenAutoCertCacheDirHasNoDataDir(t *testing.T) {
	t.Parallel()

	cacheDir := "autocert"
	cfg := &config.ConfigModule{
		Projectconfig: config.ProjectConfigOptions{
			Http: &struct {
				JwtSecret           *string                        `json:"jwtSecret,omitempty" xml:"jwtSecret,omitempty" form:"jwtSecret,omitempty"`
				JwtPublicKey        *string                        `json:"jwtPublicKey,omitempty" xml:"jwtPublicKey,omitempty" form:"jwtPublicKey,omitempty"`
				JwtOptions          *string                        `json:"jwtOptions,omitempty" xml:"jwtOptions,omitempty" form:"jwtOptions,omitempty"`
				JwtVerifyOptions    *string                        `json:"jwtVerifyOptions,omitempty" xml:"jwtVerifyOptions,omitempty" form:"jwtVerifyOptions,omitempty"`
				JwtExpiresIn        *time.Time                     `json:"jwtExpiresIn,omitempty" xml:"jwtExpiresIn,omitempty" form:"jwtExpiresIn,omitempty"`
				CookieSecret        *string                        `json:"cookieSecret,omitempty" xml:"cookieSecret,omitempty" form:"cookieSecret,omitempty"`
				AuthCors            string                         `json:"authCors,omitempty" xml:"authCors,omitempty" form:"authCors,omitempty"`
				Compression         *config.HttpCompressionOptions `json:"compression,omitempty" xml:"compression,omitempty" form:"compression,omitempty"`
				AdminCors           *string                        `json:"adminCors,omitempty" xml:"adminCors,omitempty" form:"adminCors,omitempty"`
				AuthMethodsPerActor map[string][]string            `json:"authMethodsPerActor,omitempty" xml:"authMethodsPerActor,omitempty" form:"authMethodsPerActor,omitempty"`
				AutoCert            *struct {
					CacheDir      *string  `json:"cacheDir,omitempty"`
					HostWhitelist []string `json:"hostWhitelist,omitempty"`
					Email         *string  `json:"email,omitempty"`
				} `json:"autoCert,omitempty"`
			}{
				AutoCert: &struct {
					CacheDir      *string  `json:"cacheDir,omitempty"`
					HostWhitelist []string `json:"hostWhitelist,omitempty"`
					Email         *string  `json:"email,omitempty"`
				}{
					CacheDir: &cacheDir,
				},
			},
		},
	}

	_, err := buildCertManager(cfg, "", nil)
	if err == nil {
		t.Fatalf("expected buildCertManager() to fail when cache dir is configured without data dir")
	}

	if err.Error() != "autocert cache dir requires container data dir" {
		t.Fatalf("unexpected error: %v", err)
	}
}
