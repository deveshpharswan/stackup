package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/docker"
	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/deveshpharswan/stackup/internal/hooks"
	"github.com/deveshpharswan/stackup/internal/onboard"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/printer"
	"github.com/deveshpharswan/stackup/internal/scaffold"
	dockerclient "github.com/docker/docker/client"
)

func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Validate .env and start all services in health-gated order",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p := printer.New(cmd.OutOrStdout())
			cfg := config.LoadOrEmpty(constants.DefaultConfigFile)
			o := orchestrator.New(p)

			if onboard.NeedsOnboarding(constants.DefaultEnvFile) {
				ob := onboard.New(cmd.OutOrStdout(), os.Stdin, cfg.Env.Schema)
				if err := ob.Run(constants.DefaultEnvFile, constants.DefaultExampleFile); err != nil {
					return err
				}
			}

			if !o.PreFlight(constants.DefaultEnvFile, constants.DefaultExampleFile, cfg.Env.Schema) {
				return fmt.Errorf("pre-flight validation failed")
			}

			composeServices, err := scaffold.ParseServices(constants.DefaultComposeFile)
			if err != nil {
				return fmt.Errorf("reading compose file: %w", err)
			}

			tiers, err := orchestrator.BuildTiers(composeServices)
			if err != nil {
				return err
			}

			dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
			if err != nil {
				return fmt.Errorf("connecting to Docker: %w", err)
			}
			defer dc.Close()
			checkers := buildCheckers(cfg, dc)

			// Create stackup docker client for log fetching on failure
			logClient, err := docker.NewClient()
			if err != nil {
				return fmt.Errorf("connecting to Docker for log fetching: %w", err)
			}
			defer logClient.Close()

			start := time.Now()
			for i, tier := range tiers {
				var tierDeps []string
				if i > 0 {
					tierDeps = flattenTiers(tiers[:i])
				}
				startFn := func(ctx context.Context, svcs []string) error {
					cmdArgs := append([]string{"compose", "up", "-d"}, svcs...)
					c := exec.CommandContext(ctx, "docker", cmdArgs...)
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
					return c.Run()
				}
				if err := o.StartTier(ctx, tier, tierDeps, startFn, checkers, logClient); err != nil {
					return err
				}

				// Run after_start hooks for services in this tier
				hookExec := hooks.NewExecutor(cmd.OutOrStdout())
				for _, svc := range tier {
					svcCfg, ok := cfg.Services[svc]
					if !ok || svcCfg.Hooks == nil || len(svcCfg.Hooks.AfterStart) == 0 {
						continue
					}
					if err := hookExec.RunAfterStart(ctx, svc, svcCfg.Hooks.AfterStart); err != nil {
						return fmt.Errorf("after_start hook failed for %s: %w", svc, err)
					}
				}
			}
			p.Ready(time.Since(start))
			return nil
		},
	}
}

func buildCheckers(cfg *config.Config, dc *dockerclient.Client) map[string]health.Named {
	checkers := make(map[string]health.Named)
	for name, svc := range cfg.Services {
		if svc.Health == nil {
			continue
		}
		hc := svc.Health
		timeout := parseDuration(hc.Timeout, 30)
		interval := parseDuration(hc.Interval, 2)
		switch hc.Type {
		case "http":
			checkers[name] = health.Named{Checker: health.NewHTTPChecker(hc.URL, timeout, interval), Label: "http:" + hc.URL}
		case "tcp":
			checkers[name] = health.Named{Checker: health.NewTCPChecker(hc.Host, fmt.Sprintf("%d", hc.Port), timeout, interval), Label: fmt.Sprintf("tcp:%d", hc.Port)}
		case "docker":
			checkers[name] = health.Named{Checker: health.NewDockerChecker(dc, name, timeout, interval), Label: "docker"}
		case "log":
			label := hc.Pattern
			if len(label) > 40 {
				label = label[:37] + "..."
			}
			checkers[name] = health.Named{
				Checker: health.NewLogChecker(dc, name, hc.Pattern, timeout, interval),
				Label:   "log:" + label,
			}
		}
	}
	return checkers
}

func parseDuration(s string, defaultSecs int) time.Duration {
	if s == "" {
		return time.Duration(defaultSecs) * time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Duration(defaultSecs) * time.Second
	}
	return d
}

func flattenTiers(tiers []orchestrator.Tier) []string {
	var out []string
	for _, t := range tiers {
		out = append(out, t...)
	}
	return out
}
