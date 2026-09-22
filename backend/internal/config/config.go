package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP     HTTPConfig
	Database DatabaseConfig
	ZarinHub ZarinHubConfig
	Log      LogConfig
	Audit    AuditConfig
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	AllowedOrigins []string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type ZarinHubConfig struct {
	BaseURL  string
	Username string
	Password string

	StaticToken string
	Timeout     time.Duration

	TokenSkew time.Duration
}

type LogConfig struct {
	Level  string
	Format string
}

type AuditConfig struct {
	CardHashSecret string
}

func Load(envFiles ...string) (Config, error) {
	for _, f := range envFiles {
		if err := loadEnvFile(f); err != nil {
			return Config{}, fmt.Errorf("load env file %q: %w", f, err)
		}
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Addr:            env("HTTP_ADDR", ":8080"),
			ReadTimeout:     duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			AllowedOrigins:  list("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
		Database: DatabaseConfig{
			Driver: env("DB_DRIVER", "sqlite"),
			DSN:    env("DB_DSN", "file:data/card-to-sheba.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"),
		},
		ZarinHub: ZarinHubConfig{
			BaseURL:     strings.TrimRight(env("ZARINHUB_BASE_URL", "https://zarin-hub.com/api"), "/"),
			Username:    env("ZARINHUB_USERNAME", ""),
			Password:    env("ZARINHUB_PASSWORD", ""),
			StaticToken: env("ZARINHUB_ACCESS_TOKEN", ""),
			Timeout:     duration("ZARINHUB_TIMEOUT", 15*time.Second),
			TokenSkew:   duration("ZARINHUB_TOKEN_SKEW", 60*time.Second),
		},
		Log: LogConfig{
			Level:  env("LOG_LEVEL", "info"),
			Format: env("LOG_FORMAT", "json"),
		},
		Audit: AuditConfig{
			CardHashSecret: env("AUDIT_CARD_HASH_SECRET", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var problems []string

	if c.ZarinHub.BaseURL == "" {
		problems = append(problems, "ZARINHUB_BASE_URL must not be empty")
	}
	if c.ZarinHub.StaticToken == "" && (c.ZarinHub.Username == "" || c.ZarinHub.Password == "") {
		problems = append(problems, "either ZARINHUB_ACCESS_TOKEN or both ZARINHUB_USERNAME and ZARINHUB_PASSWORD must be set")
	}
	if c.Database.DSN == "" {
		problems = append(problems, "DB_DSN must not be empty")
	}
	if c.ZarinHub.Timeout <= 0 {
		problems = append(problems, "ZARINHUB_TIMEOUT must be positive")
	}

	if len(problems) > 0 {
		return errors.New("invalid configuration: " + strings.Join(problems, "; "))
	}
	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}

	if n, err := strconv.Atoi(raw); err == nil {
		return time.Duration(n) * time.Second
	}
	return fallback
}

func list(key string, fallback []string) []string {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
