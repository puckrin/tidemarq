package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	TLS      TLSConfig      `mapstructure:"tls"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Admin    AdminConfig    `mapstructure:"admin"`
}

type ServerConfig struct {
	HTTPPort  int    `mapstructure:"http_port"`
	HTTPSPort int    `mapstructure:"https_port"`
	DataDir   string `mapstructure:"data_dir"`
	StaticDir string `mapstructure:"static_dir"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type AuthConfig struct {
	JWTSecret string        `mapstructure:"jwt_secret"`
	JWTTTL    time.Duration `mapstructure:"jwt_ttl"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.https_port", 8443)
	v.SetDefault("server.data_dir", "/data")
	v.SetDefault("server.static_dir", "/app/frontend/dist")
	v.SetDefault("database.path", "/data/tidemarq.db")
	v.SetDefault("tls.cert_file", "/data/certs/server.crt")
	v.SetDefault("tls.key_file", "/data/certs/server.key")
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_ttl", 24*time.Hour)
	v.SetDefault("admin.username", "admin")
	v.SetDefault("admin.password", "admin")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	v.SetEnvPrefix("TIDEMARQ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
