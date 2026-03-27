package apis

import "github.com/lumm2509/keel/config"

type HTTPConfig struct {
	module *config.ConfigModule
}

func HTTP(cfg *config.ConfigModule) HTTPConfig {
	return HTTPConfig{module: cfg}
}

func (cfg HTTPConfig) HTTPAllowedOrigins() []string {
	if cfg.module == nil || cfg.module.Projectconfig.Http == nil {
		return nil
	}
	return cfg.module.Projectconfig.Http.AllowedOrigins
}
