package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env    string `mapstructure:"env"`
	HTTP   HTTP   `mapstructure:"http"`
	Logger Logger `mapstructure:"logger"`
	MySQL  MySQL  `mapstructure:"mysql"`
	Redis  Redis  `mapstructure:"redis"`
	Asynq  Asynq  `mapstructure:"asynq"`
}

type HTTP struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (h HTTP) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type Logger struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
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

// Load reads configuration: defaults → yaml file (if found) → GYM_* env overrides.
func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetEnvPrefix("GYM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind env vars so they are picked up by Unmarshal even
	// without a config file or defaults (known viper AutomaticEnv caveat).
	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
		v.SetConfigName("local")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
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
	"logger.level", "logger.format",
	"mysql.host", "mysql.port", "mysql.user", "mysql.password", "mysql.database",
	"mysql.max_open_conns", "mysql.max_idle_conns", "mysql.conn_max_lifetime",
	"redis.addr", "redis.password", "redis.db",
	"asynq.concurrency",
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("env", "local")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("mysql.host", "localhost")
	v.SetDefault("mysql.port", 3306)
	v.SetDefault("mysql.max_open_conns", 25)
	v.SetDefault("mysql.max_idle_conns", 5)
	v.SetDefault("mysql.conn_max_lifetime", "5m")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("asynq.concurrency", 10)
	v.SetDefault("asynq.queues", map[string]int{"critical": 6, "default": 3, "maintenance": 1})
}

func validate(cfg *Config) error {
	if cfg.Env == "" {
		return errors.New("config: env is required")
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
	return nil
}
