package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Envs is the whitelist of allowed environments.
var Envs = []string{"dev", "prod", "test"}

// ValidateEnv checks that env is one of the allowed values.
func ValidateEnv(env string) error {
	for _, e := range Envs {
		if env == e {
			return nil
		}
	}
	if env == "" {
		return fmt.Errorf("config: GYM_ENV is not set (expected one of: %s). Example: GYM_ENV=dev make run",
			strings.Join(Envs, " | "))
	}
	return fmt.Errorf("config: invalid GYM_ENV %q (expected one of: %s)", env, strings.Join(Envs, " | "))
}

type Config struct {
	Env    string `mapstructure:"env"`
	HTTP   HTTP   `mapstructure:"http"`
	Logger Logger `mapstructure:"logger"`
	MySQL  MySQL  `mapstructure:"mysql"`
	Redis  Redis  `mapstructure:"redis"`
	Asynq  Asynq  `mapstructure:"asynq"`
	Auth   Auth   `mapstructure:"auth"`
}

type HTTP struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (h HTTP) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type Logger struct {
	Level   string   `mapstructure:"level"`
	Format  string   `mapstructure:"format"`
	Outputs []string `mapstructure:"outputs"`
	File    LogFile  `mapstructure:"file"`
}

type LogFile struct {
	Path       string `mapstructure:"path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

type MySQL struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

func (m MySQL) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true",
		m.User, m.Password, m.Host, m.Port, m.Database,
	)
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Asynq struct {
	Concurrency int            `mapstructure:"concurrency"`
	Queues      map[string]int `mapstructure:"queues"`
}

type Auth struct {
	JWTSecret        string        `mapstructure:"jwt_secret"`
	AccessTTL        time.Duration `mapstructure:"access_ttl"`
	RefreshTTL       time.Duration `mapstructure:"refresh_ttl"`
	ToleranceSec     int           `mapstructure:"tolerance_sec"`
	EmailVerifyTTL   time.Duration `mapstructure:"email_verify_ttl"`
	PasswordResetTTL time.Duration `mapstructure:"password_reset_ttl"`
	WebBaseURL       string        `mapstructure:"web_base_url"`
}

// Load reads configuration from the given file: defaults → yaml → GYM_* env overrides.
// The file must exist; the environment (env key / GYM_ENV) must pass the whitelist.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config: config file path is required")
	}

	v := viper.New()

	v.SetEnvPrefix("GYM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind env vars so they are picked up by Unmarshal even
	// for keys without defaults (known viper AutomaticEnv caveat).
	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	setDefaults(v)
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// envKeys lists all scalar config keys bound to GYM_* environment variables.
var envKeys = []string{
	"env",
	"http.host", "http.port",
	"logger.level", "logger.format", "logger.outputs",
	"logger.file.path", "logger.file.max_size_mb", "logger.file.max_age_days",
	"logger.file.max_backups", "logger.file.compress",
	"mysql.host", "mysql.port", "mysql.user", "mysql.password", "mysql.database",
	"mysql.max_open_conns", "mysql.max_idle_conns", "mysql.conn_max_lifetime",
	"redis.addr", "redis.password", "redis.db",
	"asynq.concurrency",
	"auth.jwt_secret", "auth.access_ttl", "auth.refresh_ttl", "auth.tolerance_sec",
	"auth.email_verify_ttl", "auth.password_reset_ttl", "auth.web_base_url",
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.outputs", []string{"stdout"})
	v.SetDefault("logger.file.max_size_mb", 100)
	v.SetDefault("logger.file.max_age_days", 30)
	v.SetDefault("logger.file.max_backups", 10)
	v.SetDefault("logger.file.compress", true)
	v.SetDefault("mysql.host", "localhost")
	v.SetDefault("mysql.port", 3306)
	v.SetDefault("mysql.max_open_conns", 25)
	v.SetDefault("mysql.max_idle_conns", 5)
	v.SetDefault("mysql.conn_max_lifetime", "5m")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("asynq.concurrency", 10)
	v.SetDefault("asynq.queues", map[string]int{"critical": 6, "default": 3, "maintenance": 1})
	v.SetDefault("auth.access_ttl", "15m")
	v.SetDefault("auth.refresh_ttl", "720h")
	v.SetDefault("auth.tolerance_sec", 10)
	v.SetDefault("auth.email_verify_ttl", "1h")
	v.SetDefault("auth.password_reset_ttl", "1h")
}

func validate(cfg *Config) error {
	if err := ValidateEnv(cfg.Env); err != nil {
		return err
	}
	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("config: invalid http.port %d", cfg.HTTP.Port)
	}
	if cfg.MySQL.Host == "" || cfg.MySQL.User == "" || cfg.MySQL.Database == "" {
		return errors.New("config: mysql.host, mysql.user and mysql.database are required")
	}
	if cfg.Redis.Addr == "" {
		return errors.New("config: redis.addr is required")
	}
	if len(cfg.Logger.Outputs) == 0 {
		return errors.New("config: logger.outputs must contain at least one of: stdout, file")
	}
	for _, out := range cfg.Logger.Outputs {
		if out != "stdout" && out != "file" {
			return fmt.Errorf("config: unknown logger output %q (expected stdout or file)", out)
		}
	}
	for _, out := range cfg.Logger.Outputs {
		if out == "file" && cfg.Logger.File.Path == "" {
			return errors.New("config: logger.file.path is required when output \"file\" is enabled")
		}
	}
	if cfg.Env == "prod" && cfg.Auth.JWTSecret == "" {
		return errors.New("config: auth.jwt_secret is required in prod (set GYM_AUTH_JWT_SECRET)")
	}
	if cfg.Auth.JWTSecret != "" && len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("config: auth.jwt_secret must be at least 32 bytes, got %d", len(cfg.Auth.JWTSecret))
	}
	if cfg.Auth.AccessTTL <= 0 || cfg.Auth.RefreshTTL <= 0 ||
		cfg.Auth.EmailVerifyTTL <= 0 || cfg.Auth.PasswordResetTTL <= 0 {
		return errors.New("config: auth token TTLs must be positive")
	}
	if cfg.Auth.ToleranceSec < 0 {
		return fmt.Errorf("config: auth.tolerance_sec must not be negative, got %d", cfg.Auth.ToleranceSec)
	}
	if cfg.Auth.WebBaseURL == "" {
		return errors.New("config: auth.web_base_url is required (frontend base URL for email links)")
	}
	return nil
}
