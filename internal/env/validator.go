package env

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/deveshpharswan/stackup/internal/config"
)

type ValidationError struct {
	Key     string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Key, e.Message)
}

type Result struct {
	Errors []ValidationError
}

func (r Result) Valid() bool {
	return len(r.Errors) == 0
}

func Validate(envFile, exampleFile string, schema map[string]config.EnvVar) Result {
	result, _ := ValidateWithDefaults(envFile, exampleFile, schema)
	return result
}

// ValidateWithDefaults validates env vars and injects schema defaults for missing keys.
// It returns the validation result and a map of keys that were filled with defaults.
func ValidateWithDefaults(envFile, exampleFile string, schema map[string]config.EnvVar) (Result, map[string]string) {
	var result Result
	injected := make(map[string]string)

	envVars, err := godotenv.Read(envFile)
	if err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Key:     "env",
			Message: "could not read " + envFile + ": " + err.Error(),
		})
		return result, injected
	}
	example, _ := godotenv.Read(exampleFile)

	// Inject defaults for missing keys that have a default in schema
	for key, rule := range schema {
		if _, ok := envVars[key]; !ok && rule.Default != "" {
			envVars[key] = rule.Default
			injected[key] = rule.Default
		}
	}

	for key := range example {
		if _, ok := envVars[key]; !ok {
			result.Errors = append(result.Errors, ValidationError{
				Key:     key,
				Message: "missing (required by .env.example)",
			})
		}
	}

	for key, rule := range schema {
		val, ok := envVars[key]
		if !ok {
			if rule.Required {
				result.Errors = append(result.Errors, ValidationError{
					Key:     key,
					Message: "required but not set",
				})
			}
			continue
		}
		if err := validateType(key, val, rule.Type); err != nil {
			result.Errors = append(result.Errors, *err)
		}
	}

	return result, injected
}

func validateType(key, val, typ string) *ValidationError {
	switch typ {
	case "int":
		if _, err := strconv.Atoi(val); err != nil {
			return &ValidationError{Key: key, Message: fmt.Sprintf("expected int, got %q", val)}
		}
	case "bool":
		lower := strings.ToLower(val)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" {
			return &ValidationError{Key: key, Message: fmt.Sprintf("expected bool, got %q", val)}
		}
	case "url":
		u, err := url.Parse(val)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return &ValidationError{Key: key, Message: fmt.Sprintf("expected valid URL, got %q", val)}
		}
	}
	return nil
}
