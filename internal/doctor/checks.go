package doctor

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/scaffold"
)

// CheckPortConflicts verifies that ports configured in stackup.yml health checks are available.
func CheckPortConflicts(ctx context.Context, opts *Options) []Finding {
	var findings []Finding

	cfg, err := config.Load(opts.ConfigFile)
	if err != nil {
		// No config file — skip this check silently.
		return nil
	}

	for name, svc := range cfg.Services {
		if svc.Health == nil || svc.Health.Port == 0 {
			continue
		}
		addr := fmt.Sprintf(":%d", svc.Health.Port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			fix := fmt.Sprintf("lsof -i :%d", svc.Health.Port)
			if runtime.GOOS == "windows" {
				fix = fmt.Sprintf("netstat -ano | findstr :%d", svc.Health.Port)
			}
			findings = append(findings, Finding{
				Severity: SeverityError,
				Title:    fmt.Sprintf("Port %d is already in use", svc.Health.Port),
				Detail:   fmt.Sprintf("Service %q expects port %d but it is occupied", name, svc.Health.Port),
				Fix:      fix,
				Service:  name,
			})
		} else {
			ln.Close()
		}
	}
	return findings
}

// CheckCrashLoops detects services stuck in restarting or exited state.
func CheckCrashLoops(ctx context.Context, opts *Options) []Finding {
	var findings []Finding

	args := []string{"compose"}
	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}
	args = append(args, "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Status}}")

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		// Docker not available or compose not running — skip.
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		svcName := strings.TrimSpace(parts[0])
		state := strings.ToLower(strings.TrimSpace(parts[1]))

		if state == "restarting" || state == "exited" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Title:    fmt.Sprintf("Service %q is in %s state", svcName, state),
				Detail:   "The container may be crash-looping or has stopped unexpectedly",
				Fix:      fmt.Sprintf("docker compose logs %s --tail=50", svcName),
				Service:  svcName,
			})
		}
	}
	return findings
}

// CheckEnvDrift detects keys present in .env.example but missing from .env.
func CheckEnvDrift(ctx context.Context, opts *Options) []Finding {
	var findings []Finding

	envFile := opts.EnvFile
	exampleFile := opts.ExampleFile
	if envFile == "" {
		envFile = ".env"
	}
	if exampleFile == "" {
		exampleFile = ".env.example"
	}

	envVars, err := godotenv.Read(envFile)
	if err != nil {
		// No .env file — nothing to compare.
		return nil
	}

	exampleVars, err := godotenv.Read(exampleFile)
	if err != nil {
		// No example file — skip.
		return nil
	}

	var missing []string
	for key := range exampleVars {
		if _, exists := envVars[key]; !exists {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Title:    "Environment drift detected",
			Detail:   fmt.Sprintf("Keys in %s but not in %s: %s", exampleFile, envFile, strings.Join(missing, ", ")),
			Fix:      fmt.Sprintf("Add missing keys to %s", envFile),
		})
	}
	return findings
}

// CheckContainerStatus reports running containers as OK findings.
func CheckContainerStatus(ctx context.Context, opts *Options) []Finding {
	var findings []Finding

	args := []string{"compose"}
	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}
	args = append(args, "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Status}}")

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		svcName := strings.TrimSpace(parts[0])
		state := strings.ToLower(strings.TrimSpace(parts[1]))

		if state == "running" {
			status := ""
			if len(parts) == 3 {
				status = strings.TrimSpace(parts[2])
			}
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Title:    fmt.Sprintf("Service %q is running", svcName),
				Detail:   status,
				Service:  svcName,
			})
		}
	}
	return findings
}

// CheckLocalhostMisuse warns when .env contains localhost references that should
// use Docker service names inside containers.
func CheckLocalhostMisuse(ctx context.Context, opts *Options) []Finding {
	var findings []Finding

	envFile := opts.EnvFile
	if envFile == "" {
		envFile = ".env"
	}

	envVars, err := godotenv.Read(envFile)
	if err != nil {
		return nil
	}

	composeFile := opts.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	services, err := scaffold.ParseServices(composeFile)
	if err != nil {
		return nil
	}

	// Build a set of service names from docker-compose.
	serviceNames := make(map[string]bool, len(services))
	for name := range services {
		serviceNames[name] = true
	}

	for envKey, envVal := range envVars {
		lower := strings.ToLower(envVal)
		for svcName := range serviceNames {
			port := guessServicePort(svcName)
			if port == "" {
				continue
			}
			localhostPatterns := []string{
				"localhost:" + port,
				"127.0.0.1:" + port,
			}
			for _, pattern := range localhostPatterns {
				if strings.Contains(lower, pattern) {
					findings = append(findings, Finding{
						Severity: SeverityWarning,
						Title:    fmt.Sprintf("Localhost reference in %s may not work inside containers", envKey),
						Detail:   fmt.Sprintf("Found %q — inside Docker, use the service name %q instead of localhost", pattern, svcName),
						Fix:      fmt.Sprintf("Replace localhost:%s with %s:%s in %s", port, svcName, port, envKey),
						Service:  svcName,
					})
				}
			}
		}
	}
	return findings
}

// guessServicePort maps well-known service names to their default ports.
func guessServicePort(svcName string) string {
	name := strings.ToLower(svcName)
	defaults := map[string]string{
		"postgres":      "5432",
		"postgresql":    "5432",
		"pg":            "5432",
		"mysql":         "3306",
		"mariadb":       "3306",
		"redis":         "6379",
		"mongo":         "27017",
		"mongodb":       "27017",
		"elasticsearch": "9200",
		"rabbitmq":      "5672",
		"nats":          "4222",
		"kafka":         "9092",
		"minio":         "9000",
		"memcached":     "11211",
	}
	if port, ok := defaults[name]; ok {
		return port
	}
	return ""
}
