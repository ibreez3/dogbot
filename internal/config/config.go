package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/viper"
)

var (
	instance *Config
	once     sync.Once
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Database DatabaseConfig `mapstructure:"database"`
	Features FeaturesConfig `mapstructure:"features"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	Secret           string   `mapstructure:"secret"`
	Tokens           []string `mapstructure:"tokens"`
	TokenRequired    bool     `mapstructure:"token_required"`
	DeviceCheck      bool     `mapstructure:"device_check"`
	AllowedDeviceIDs []string `mapstructure:"allowed_device_ids"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"` // json or console
	Output     string `mapstructure:"output"` // stdout or file path
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type     string `mapstructure:"type"`     // sqlite, postgres, mysql
	Path     string `mapstructure:"path"`     // for sqlite
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// FeaturesConfig represents feature flags
type FeaturesConfig struct {
	Events      bool `mapstructure:"events"`
	Presence    bool `mapstructure:"presence"`
	HealthCheck bool `mapstructure:"health_check"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	var err error
	once.Do(func() {
		instance, err = loadConfig(configPath)
	})
	return instance, err
}

// loadConfig loads configuration from file
func loadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// If config path is provided, read from file
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Try to find config file in default locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/openclaw")

		// Read from file if exists
		if err := v.ReadInConfig(); err != nil {
			// Config file doesn't exist, that's okay
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read config: %w", err)
			}
		}
	}

	// Read from environment variables
	v.SetEnvPrefix("OPENCLAW")
	v.AutomaticEnv()

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 18790)
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("server.shutdown_timeout", 10)

	// Auth defaults
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.secret", "change-me-in-production")
	v.SetDefault("auth.tokens", []string{})
	v.SetDefault("auth.token_required", false)
	v.SetDefault("auth.device_check", false)
	v.SetDefault("auth.allowed_device_ids", []string{})

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 3)
	v.SetDefault("logging.max_age", 28)

	// Database defaults
	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.path", "./data/openclaw.db")

	// Features defaults
	v.SetDefault("features.events", true)
	v.SetDefault("features.presence", true)
	v.SetDefault("features.health_check", true)
}

// Get returns the singleton config instance
func Get() *Config {
	if instance == nil {
		// Load with default path if not loaded yet
		cfg, err := Load("")
		if err != nil {
			panic(fmt.Sprintf("failed to load config: %v", err))
		}
		instance = cfg
	}
	return instance
}

// GetAddr returns the server address
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// ValidateToken validates if the given token is allowed
func (c *Config) ValidateToken(token string) bool {
	if !c.Auth.Enabled || !c.Auth.TokenRequired {
		return true
	}

	for _, t := range c.Auth.Tokens {
		if t == token {
			return true
		}
	}
	return false
}

// IsDeviceAllowed checks if a device ID is allowed
func (c *Config) IsDeviceAllowed(deviceID string) bool {
	if !c.Auth.Enabled || !c.Auth.DeviceCheck || len(c.Auth.AllowedDeviceIDs) == 0 {
		return true
	}

	for _, id := range c.Auth.AllowedDeviceIDs {
		if id == deviceID {
			return true
		}
	}
	return false
}

// Save saves the current configuration to a file
func (c *Config) Save(path string) error {
	v := viper.New()
	// 直接设置配置值
	if err := v.Unmarshal(c); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return v.WriteConfigAs(path)
}

// GetEnv returns the current environment (dev, prod, etc.)
func GetEnv() string {
	env := os.Getenv("OPENCLAW_ENV")
	if env == "" {
		env = "development"
	}
	return env
}

// IsProd returns true if running in production
func IsProd() bool {
	return GetEnv() == "production"
}

// IsDev returns true if running in development
func IsDev() bool {
	return GetEnv() == "development"
}
