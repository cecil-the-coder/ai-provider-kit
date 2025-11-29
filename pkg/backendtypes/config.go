package backendtypes

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BackendConfig defines the configuration for the backend server
type BackendConfig struct {
	Server     ServerConfig                      `yaml:"server"`
	Auth       AuthConfig                        `yaml:"auth"`
	Logging    LoggingConfig                     `yaml:"logging"`
	CORS       CORSConfig                        `yaml:"cors"`
	Providers  map[string]*types.ProviderConfig  `yaml:"providers"`
	Extensions map[string]ExtensionConfig        `yaml:"extensions"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Version         string        `yaml:"version"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type AuthConfig struct {
	Enabled     bool     `yaml:"enabled"`
	APIPassword string   `yaml:"api_password"`
	APIKeyEnv   string   `yaml:"api_key_env"`
	PublicPaths []string `yaml:"public_paths"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
}

type ExtensionConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}
