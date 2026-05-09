package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  string             `yaml:"version"`
	Env      EnvConfig          `yaml:"env"`
	Services map[string]Service `yaml:"services"`
	Commands map[string]Command `yaml:"commands"`
}

type EnvConfig struct {
	Schema map[string]EnvVar `yaml:"schema"`
}

type EnvVar struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

type Service struct {
	Health *HealthCheck `yaml:"health"`
	Hooks  *Hooks       `yaml:"hooks"`
}

type Hooks struct {
	AfterStart []HookAction `yaml:"after_start"`
}

type HookAction struct {
	Name    string `yaml:"name"`
	Service string `yaml:"service"`
	Run     string `yaml:"run"`
}

type HealthCheck struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Pattern  string `yaml:"pattern"`
	Timeout  string `yaml:"timeout"`
	Interval string `yaml:"interval"`
}

type Command struct {
	Service string `yaml:"service"`
	Run     string `yaml:"run"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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
