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
		deps[name] = parseDependsOn(svc.DependsOn)
	}
	return deps, nil
}

func parseDependsOn(v interface{}) []string {
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
		b.WriteString(fmt.Sprintf("  %s:\n    health:\n      type: tcp  # TODO: configure health check\n", name))
	}

	b.WriteString("\ncommands: {}\n")
	return b.String(), nil
}
