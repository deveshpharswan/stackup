// Package scaffold parses docker-compose.yml files and generates stackup.yml configs.
package scaffold

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	DependsOn interface{} `yaml:"depends_on"`
	Image     string      `yaml:"image"`
	Profiles  []string    `yaml:"profiles"`
}

// Dependency represents a service dependency with its condition.
type Dependency struct {
	Service   string
	Condition string // "service_started", "service_healthy", "service_completed_successfully"
}

// HealthDefault holds the detected health check configuration for a known image.
type HealthDefault struct {
	Type string
	Host string
	Port int
}

var knownImages = map[string]HealthDefault{
	"postgres":      {Type: "tcp", Host: "localhost", Port: 5432},
	"redis":         {Type: "tcp", Host: "localhost", Port: 6379},
	"mysql":         {Type: "tcp", Host: "localhost", Port: 3306},
	"mariadb":       {Type: "tcp", Host: "localhost", Port: 3306},
	"mongo":         {Type: "tcp", Host: "localhost", Port: 27017},
	"elasticsearch": {Type: "tcp", Host: "localhost", Port: 9200},
	"rabbitmq":      {Type: "tcp", Host: "localhost", Port: 5672},
	"kafka":         {Type: "tcp", Host: "localhost", Port: 9092},
	"nginx":         {Type: "http", Host: "localhost", Port: 80},
	"memcached":     {Type: "tcp", Host: "localhost", Port: 11211},
	"nats":          {Type: "tcp", Host: "localhost", Port: 4222},
	"minio":         {Type: "tcp", Host: "localhost", Port: 9000},
	"consul":        {Type: "http", Host: "localhost", Port: 8500},
	"vault":         {Type: "tcp", Host: "localhost", Port: 8200},
	"etcd":          {Type: "tcp", Host: "localhost", Port: 2379},
	"zookeeper":     {Type: "tcp", Host: "localhost", Port: 2181},
	"cockroachdb":   {Type: "tcp", Host: "localhost", Port: 26257},
	"clickhouse":    {Type: "tcp", Host: "localhost", Port: 9000},
}

// DetectHealthDefault checks if the image name contains any known pattern.
// Returns nil if no match found.
func DetectHealthDefault(image string) *HealthDefault {
	lower := strings.ToLower(image)
	for pattern, def := range knownImages {
		if strings.Contains(lower, pattern) {
			d := def // copy
			return &d
		}
	}
	return nil
}

// ParseServices reads a docker-compose.yml and returns a map of service name to its dependency list.
func ParseServices(composeFilePath string) (map[string][]string, error) {
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading compose file: %w", err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing compose file: %w", err)
	}
	deps := make(map[string][]string, len(cf.Services))
	for name, svc := range cf.Services {
		deps[name] = parseDependsOnNames(svc.DependsOn)
	}
	return deps, nil
}

// ParseServicesRich reads a docker-compose.yml and returns dependencies with conditions.
// This allows stackup to understand if compose already expects service_healthy checks.
func ParseServicesRich(composeFilePath string) (map[string][]Dependency, error) {
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading compose file: %w", err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing compose file: %w", err)
	}
	deps := make(map[string][]Dependency, len(cf.Services))
	for name, svc := range cf.Services {
		deps[name] = parseDependsOnRich(svc.DependsOn)
	}
	return deps, nil
}

func parseDependsOnNames(v interface{}) []string {
	switch val := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case map[string]interface{}:
		out := make([]string, 0, len(val))
		for k := range val {
			out = append(out, k)
		}
		return out
	}
	return nil
}

func parseDependsOnRich(v interface{}) []Dependency {
	switch val := v.(type) {
	case []interface{}:
		out := make([]Dependency, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, Dependency{Service: s, Condition: "service_started"})
			}
		}
		return out
	case map[string]interface{}:
		out := make([]Dependency, 0, len(val))
		for k, v := range val {
			condition := "service_started"
			if m, ok := v.(map[string]interface{}); ok {
				if c, ok := m["condition"].(string); ok {
					condition = c
				}
			}
			out = append(out, Dependency{Service: k, Condition: condition})
		}
		return out
	}
	return nil
}

// Generate creates a stackup.yml configuration from a docker-compose.yml file,
// auto-detecting health check types for known images.
func Generate(composeFilePath, exampleFile string) (string, error) {
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return "", fmt.Errorf("reading compose file: %w", err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return "", fmt.Errorf("parsing compose file: %w", err)
	}

	envKeys, _ := godotenv.Read(exampleFile)

	var b strings.Builder
	b.WriteString("version: \"1\"\n")

	if len(envKeys) > 0 {
		b.WriteString("\nenv:\n  schema:\n")
		sortedKeys := make([]string, 0, len(envKeys))
		for k := range envKeys {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)
		for _, key := range sortedKeys {
			b.WriteString(fmt.Sprintf("    %s:\n      required: true\n", key))
		}
	}

	b.WriteString("\nservices:\n")
	svcNames := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := cf.Services[name]
		def := DetectHealthDefault(svc.Image)
		if def != nil {
			b.WriteString(fmt.Sprintf("  %s:\n    health:\n      type: %s\n", name, def.Type))
			if def.Type == "tcp" {
				b.WriteString(fmt.Sprintf("      host: %s\n      port: %d\n", def.Host, def.Port))
			} else if def.Type == "http" {
				b.WriteString(fmt.Sprintf("      url: http://%s:%d/\n", def.Host, def.Port))
			}
		} else {
			b.WriteString(fmt.Sprintf("  %s:\n    health:\n      type: tcp  # TODO: set correct health check\n", name))
		}
	}

	b.WriteString("\ncommands: {}\n")
	return b.String(), nil
}
