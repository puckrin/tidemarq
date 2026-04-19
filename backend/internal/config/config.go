package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
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

	v.SetDefault("server.http_port", 8716)
	v.SetDefault("server.https_port", 8717)
	v.SetDefault("server.data_dir", "/data")
	v.SetDefault("server.static_dir", "/app/frontend/dist")
	v.SetDefault("database.path", "/data/tidemarq.db")
	v.SetDefault("tls.cert_file", "/data/certs/server.crt")
	v.SetDefault("tls.key_file", "/data/certs/server.key")
	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.jwt_ttl", 24*time.Hour)
	v.SetDefault("admin.username", "admin")
	v.SetDefault("admin.password", "admin123")

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

// EnsureJWTSecret guarantees cfg.Auth.JWTSecret is set to a non-empty value.
// If the config/env already provides one it is used unchanged.
// Otherwise the secret is loaded from (or generated and saved to)
// <data_dir>/.jwt_secret so it survives restarts without user intervention.
func EnsureJWTSecret(cfg *Config) error {
	if cfg.Auth.JWTSecret != "" {
		return nil
	}

	secretFile := filepath.Join(cfg.Server.DataDir, ".jwt_secret")

	data, err := os.ReadFile(secretFile)
	if err == nil {
		cfg.Auth.JWTSecret = strings.TrimSpace(string(data))
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("reading jwt secret file: %w", err)
	}

	// Generate a new 32-byte random secret and persist it.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generating jwt secret: %w", err)
	}
	secret := base64.StdEncoding.EncodeToString(raw)

	if err := os.MkdirAll(cfg.Server.DataDir, 0o700); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}
	if err := os.WriteFile(secretFile, []byte(secret+"\n"), 0o600); err != nil {
		return fmt.Errorf("writing jwt secret file: %w", err)
	}

	cfg.Auth.JWTSecret = secret
	return nil
}
