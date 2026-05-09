// Package config handles loading and validating the stackup.yml configuration file.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level stackup.yml structure.
type Config struct {
	Version  string             `yaml:"version"`
	Env      EnvConfig          `yaml:"env"`
	Services map[string]Service `yaml:"services"`
	Commands map[string]Command `yaml:"commands"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Profile defines a named subset of services to start.
type Profile struct {
	Services []string `yaml:"services"`
}

// EnvConfig holds the environment variable schema definitions.
type EnvConfig struct {
	Schema map[string]EnvVar `yaml:"schema"`
}

// EnvVar describes a single environment variable's constraints.
type EnvVar struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

// Service defines the health check and lifecycle hooks for a compose service.
type Service struct {
	Health *HealthCheck `yaml:"health"`
	Hooks  *Hooks       `yaml:"hooks"`
}

// Hooks contains lifecycle hook definitions for a service.
type Hooks struct {
	AfterStart []HookAction `yaml:"after_start"`
}

// HookAction describes a single hook command to execute inside a container.
type HookAction struct {
	Name    string `yaml:"name"`
	Service string `yaml:"service"`
	Run     string `yaml:"run"`
}

// HealthCheck defines how to verify a service is ready.
type HealthCheck struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Pattern  string `yaml:"pattern"`
	Timeout  string `yaml:"timeout"`
	Interval string `yaml:"interval"`
}

// Command defines a named command that runs inside a service container.
type Command struct {
	Service string `yaml:"service"`
	Run     string `yaml:"run"`
}

// Load reads and validates a stackup.yml file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid stackup.yml: %w", err)
	}
	return &cfg, nil
}

var validHealthTypes = map[string]bool{
	"http": true, "tcp": true, "docker": true, "log": true,
}

// Validate checks the config for structural correctness.
func (c *Config) Validate() error {
	for name, svc := range c.Services {
		if svc.Health == nil {
			continue
		}
		hc := svc.Health
		if !validHealthTypes[hc.Type] {
			return fmt.Errorf("service %q: unknown health check type %q (must be http, tcp, docker, or log)", name, hc.Type)
		}
		switch hc.Type {
		case "http":
			if hc.URL == "" {
				return fmt.Errorf("service %q: http health check requires 'url' field", name)
			}
		case "tcp":
			if hc.Host == "" || hc.Port == 0 {
				return fmt.Errorf("service %q: tcp health check requires 'host' and 'port' fields", name)
			}
			if hc.Port < 1 || hc.Port > 65535 {
				return fmt.Errorf("service %q: port must be between 1 and 65535, got %d", name, hc.Port)
			}
		case "log":
			if hc.Pattern == "" {
				return fmt.Errorf("service %q: log health check requires 'pattern' field", name)
			}
		}
		if hc.Timeout != "" {
			if _, err := time.ParseDuration(hc.Timeout); err != nil {
				return fmt.Errorf("service %q: invalid timeout %q: %w", name, hc.Timeout, err)
			}
		}
		if hc.Interval != "" {
			if _, err := time.ParseDuration(hc.Interval); err != nil {
				return fmt.Errorf("service %q: invalid interval %q: %w", name, hc.Interval, err)
			}
		}
	}
	return nil
}

// LoadOrEmpty returns an empty Config when the file does not exist.
// Allows projects that haven't added stackup.yml yet to still use the tool.
func LoadOrEmpty(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		return &Config{}
	}
	return cfg
}

// ProfileServices returns the service names for a given profile.
// Returns an error if the profile is not defined.
func (c *Config) ProfileServices(name string) ([]string, error) {
	if c.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined in config")
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return p.Services, nil
}
