package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
	var (
		profile string
		partial bool
		only    string
	)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Validate .env and start all services in health-gated order",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()
			p := printer.New(cmd.OutOrStdout())
			cfg := config.LoadOrEmpty(constants.DefaultConfigFile)
			o := orchestrator.New(p)

			if onboard.NeedsOnboarding(constants.DefaultEnvFile) {
				ob := onboard.New(cmd.OutOrStdout(), os.Stdin, cfg.Env.Schema)
				if err := ob.Run(constants.DefaultEnvFile, constants.DefaultExampleFile); err != nil {
					return err
				}
			}

			ok, injected := o.PreFlight(constants.DefaultEnvFile, constants.DefaultExampleFile, cfg.Env.Schema)
			if !ok {
				return fmt.Errorf("pre-flight validation failed")
			}

			// Build environment for child processes: inherit current env + injected defaults
			childEnv := os.Environ()
			for key, val := range injected {
				childEnv = append(childEnv, key+"="+val)
			}

			composeServices, err := scaffold.ParseServices(constants.DefaultComposeFile)
			if err != nil {
				return fmt.Errorf("reading compose file: %w", err)
			}

			// Filter by profile if specified
			if profile != "" {
				profileSvcs, err := cfg.ProfileServices(profile)
				if err != nil {
					return err
				}
				composeServices = filterWithDeps(composeServices, profileSvcs)
			}

			// Filter by --only if specified
			if only != "" {
				parts := strings.Split(only, ",")
				var onlySvcs []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						onlySvcs = append(onlySvcs, p)
					}
				}
				if len(onlySvcs) == 0 {
					return fmt.Errorf("--only requires at least one service name")
				}
				for _, svc := range onlySvcs {
					if _, exists := composeServices[svc]; !exists {
						return fmt.Errorf("unknown service %q (not in docker-compose.yml)", svc)
					}
				}
				composeServices = filterWithDeps(composeServices, onlySvcs)
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

			startFn := func(ctx context.Context, svcs []string) error {
				cmdArgs := append([]string{"compose", "up", "-d"}, svcs...)
				c := exec.CommandContext(ctx, "docker", cmdArgs...)
				c.Env = childEnv
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				return c.Run()
			}

			start := time.Now()

			if partial {
				return runPartial(ctx, o, tiers, composeServices, startFn, checkers, logClient, cfg, cmd, p, start)
			}

			var allResults []printer.ServiceResult
			for i, tier := range tiers {
				var tierDeps []string
				if i > 0 {
					tierDeps = flattenTiers(tiers[:i])
				}
				results, err := o.StartTierWithResults(ctx, tier, tierDeps, startFn, checkers, logClient)
				allResults = append(allResults, results...)
				if err != nil {
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
			elapsed := time.Since(start)
			if len(allResults) > 0 {
				p.SummaryTable(allResults, elapsed)
			} else {
				p.Ready(elapsed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Start only services in the named profile (includes dependencies)")
	cmd.Flags().BoolVar(&partial, "partial", false, "Continue starting independent services even if some fail")
	cmd.Flags().StringVar(&only, "only", "", "Start only the named services and their dependencies (comma-separated)")
	return cmd
}

func runPartial(ctx context.Context, o *orchestrator.Orchestrator, tiers []orchestrator.Tier, composeServices map[string][]string, startFn func(context.Context, []string) error, checkers map[string]health.Named, logClient *docker.Client, cfg *config.Config, cmd *cobra.Command, p *printer.Printer, start time.Time) error {
	failed := make(map[string]bool)
	var totalServices, healthyCount int

	for i, tier := range tiers {
		// Filter out services whose dependencies failed
		var viableTier orchestrator.Tier
		for _, svc := range tier {
			skip := false
			for _, dep := range composeServices[svc] {
				if failed[dep] {
					skip = true
					break
				}
			}
			if skip {
				failed[svc] = true
				p.ServiceFailed(svc, fmt.Errorf("skipped: dependency failed"))
			} else {
				viableTier = append(viableTier, svc)
			}
		}

		totalServices += len(tier)
		if len(viableTier) == 0 {
			continue
		}

		var tierDeps []string
		if i > 0 {
			tierDeps = flattenTiers(tiers[:i])
		}

		tierFailed, err := o.StartTierPartial(ctx, viableTier, tierDeps, startFn, checkers, logClient)
		if err != nil {
			return err
		}
		for _, f := range tierFailed {
			failed[f] = true
		}
		healthyCount += len(viableTier) - len(tierFailed)

		// Run after_start hooks for healthy services
		hookExec := hooks.NewExecutor(cmd.OutOrStdout())
		for _, svc := range viableTier {
			if failed[svc] {
				continue
			}
			svcCfg, ok := cfg.Services[svc]
			if !ok || svcCfg.Hooks == nil || len(svcCfg.Hooks.AfterStart) == 0 {
				continue
			}
			if err := hookExec.RunAfterStart(ctx, svc, svcCfg.Hooks.AfterStart); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! hook failed for %s: %v\n", svc, err)
			}
		}
	}

	if len(failed) == 0 {
		p.Ready(time.Since(start))
		return nil
	}

	pct := (healthyCount * 100) / totalServices
	fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ Stack %d%% ready (%d/%d services healthy)\n", pct, healthyCount, totalServices)
	failedNames := make([]string, 0, len(failed))
	for name := range failed {
		failedNames = append(failedNames, name)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Failed: %s\n", strings.Join(failedNames, ", "))
	fmt.Fprintf(cmd.OutOrStdout(), "  Duration: %s\n", formatUpDuration(time.Since(start)))
	return &ExitError{Code: 3, Message: fmt.Sprintf("partial success: %d/%d services healthy", healthyCount, totalServices)}
}

func formatUpDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
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

// filterWithDeps returns the subset of allDeps containing only the named services
// and their transitive dependencies.
func filterWithDeps(allDeps map[string][]string, services []string) map[string][]string {
	needed := make(map[string]bool)
	var resolve func(string)
	resolve = func(svc string) {
		if needed[svc] {
			return
		}
		needed[svc] = true
		for _, dep := range allDeps[svc] {
			resolve(dep)
		}
	}
	for _, svc := range services {
		resolve(svc)
	}

	filtered := make(map[string][]string, len(needed))
	for svc := range needed {
		var deps []string
		for _, d := range allDeps[svc] {
			if needed[d] {
				deps = append(deps, d)
			}
		}
		filtered[svc] = deps
	}
	return filtered
}
