package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	dockerclient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
)

// ExitError wraps an exit code for CLI commands that need non-zero exits
// without calling os.Exit directly (which breaks testability and defers).
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }

type checkResult struct {
	Stack    string         `json:"stack"`
	Healthy  bool           `json:"healthy"`
	Services []serviceCheck `json:"services"`
}

type serviceCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func newCheckCmd() *cobra.Command {
	var (
		serviceName string
		format      string
		quiet       bool
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check health of services (CI-friendly, exits 0=healthy, 2=unhealthy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg := config.LoadOrEmpty(constants.DefaultConfigFile)

			dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
			if err != nil {
				return fmt.Errorf("connecting to Docker: %w", err)
			}
			defer dc.Close()

			checkers := buildCheckers(cfg, dc)

			// Determine which services to check
			var targets []string
			if serviceName != "" {
				targets = []string{serviceName}
			} else {
				for name := range checkers {
					targets = append(targets, name)
				}
			}

			result := checkResult{Stack: "stackup", Healthy: true}

			for _, svc := range targets {
				named, ok := checkers[svc]
				if !ok {
					result.Services = append(result.Services, serviceCheck{
						Name:    svc,
						Status:  "no_check",
						Message: "no health check configured",
					})
					continue
				}
				if err := named.Checker.Check(ctx); err != nil {
					result.Healthy = false
					result.Services = append(result.Services, serviceCheck{
						Name:    svc,
						Status:  "unhealthy",
						Message: err.Error(),
					})
				} else {
					result.Services = append(result.Services, serviceCheck{
						Name:   svc,
						Status: "healthy",
					})
				}
			}

			// Output based on format
			w := cmd.OutOrStdout()
			switch format {
			case "json":
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
			default:
				if !quiet {
					for _, s := range result.Services {
						switch s.Status {
						case "healthy":
							fmt.Fprintf(w, "  ✓ %-12s healthy\n", s.Name)
						case "unhealthy":
							fmt.Fprintf(w, "  ✗ %-12s unhealthy: %s\n", s.Name, s.Message)
						default:
							fmt.Fprintf(w, "  ? %-12s %s\n", s.Name, s.Message)
						}
					}
				}
			}

			if !result.Healthy {
				return &ExitError{Code: 2, Message: "one or more services unhealthy"}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "", "Check a single service by name")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress output, exit code only")

	return cmd
}
