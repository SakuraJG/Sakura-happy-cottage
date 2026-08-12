package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
	Storage struct {
		UploadDir      string `yaml:"upload_dir"`
		MaxUploadBytes int64  `yaml:"max_upload_bytes"`
	} `yaml:"storage"`
	Auth struct {
		CookieName              string `yaml:"cookie_name"`
		CookieSecure            bool   `yaml:"cookie_secure"`
		SessionTTLHours         int    `yaml:"session_ttl_hours"`
		BcryptCost              int    `yaml:"bcrypt_cost"`
		EmailVerifyTTLHours     int    `yaml:"email_verify_ttl_hours"`
		PasswordResetTTLMinutes int    `yaml:"password_reset_ttl_minutes"`
	} `yaml:"auth"`
	Admin struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	applyEnvironment(&cfg)
	if cfg.Database.Host == "" || cfg.Database.Name == "" || cfg.Database.User == "" || cfg.Database.Password == "" {
		return Config{}, errors.New("database.host, database.name, database.user and database.password are required; set the password in config.yaml or SAKURA_HOME_DATABASE_PASSWORD")
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 13888
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Storage.UploadDir == "" {
		cfg.Storage.UploadDir = "./data/uploads"
	}
	if cfg.Storage.MaxUploadBytes <= 0 {
		cfg.Storage.MaxUploadBytes = 25 * 1024 * 1024
	}
	if cfg.Auth.CookieName == "" {
		cfg.Auth.CookieName = "sakura_home_session"
	}
	if cfg.Auth.SessionTTLHours <= 0 {
		cfg.Auth.SessionTTLHours = 24 * 7
	}
	if cfg.Auth.BcryptCost == 0 {
		cfg.Auth.BcryptCost = 12
	}
	if cfg.Auth.BcryptCost < 10 || cfg.Auth.BcryptCost > 14 {
		return Config{}, errors.New("auth.bcrypt_cost must be between 10 and 14")
	}
	if cfg.Auth.EmailVerifyTTLHours <= 0 {
		cfg.Auth.EmailVerifyTTLHours = 24
	}
	if cfg.Auth.PasswordResetTTLMinutes <= 0 {
		cfg.Auth.PasswordResetTTLMinutes = 30
	}
	if cfg.Admin.Username == "" {
		cfg.Admin.Username = "admin"
	}
	if !usernamePattern.MatchString(cfg.Admin.Username) {
		return Config{}, errors.New("admin.username must be 3-32 letters, numbers, or underscores")
	}
	if cfg.Admin.Password != "" {
		if len([]rune(cfg.Admin.Password)) < 12 {
			return Config{}, errors.New("bootstrap admin password must contain at least 12 characters")
		}
		if len(cfg.Admin.Password) > 72 {
			return Config{}, errors.New("bootstrap admin password must not exceed 72 bytes")
		}
		if insecureBootstrapPassword(cfg.Admin.Password) {
			return Config{}, errors.New("bootstrap admin password is a known placeholder or default value")
		}
	}
	return cfg, nil
}

func applyEnvironment(cfg *Config) {
	if value, ok := os.LookupEnv("SAKURA_HOME_DATABASE_PASSWORD"); ok {
		cfg.Database.Password = value
	}
	if value, ok := os.LookupEnv("SAKURA_HOME_ADMIN_USERNAME"); ok {
		cfg.Admin.Username = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("SAKURA_HOME_ADMIN_PASSWORD"); ok {
		cfg.Admin.Password = value
	}
}

func insecureBootstrapPassword(password string) bool {
	switch strings.ToLower(strings.TrimSpace(password)) {
	case "admin", "adminadmin", "password", "changeme", "change_me", "change-me", "replace_me", "replace-me":
		return true
	default:
		return false
	}
}
