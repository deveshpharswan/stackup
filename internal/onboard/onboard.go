// Package onboard provides interactive .env creation for new team members.
package onboard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/deveshpharswan/stackup/internal/config"
)

// NeedsOnboarding returns true if the .env file does not exist.
func NeedsOnboarding(envFile string) bool {
	_, err := os.Stat(envFile)
	return os.IsNotExist(err)
}

// Onboarder walks new developers through creating a .env file interactively.
type Onboarder struct {
	w      io.Writer
	r      io.Reader
	schema map[string]config.EnvVar
}

// New creates an Onboarder that reads from r, writes prompts to w,
// and uses the provided schema for defaults and descriptions.
func New(w io.Writer, r io.Reader, schema map[string]config.EnvVar) *Onboarder {
	return &Onboarder{
		w:      w,
		r:      r,
		schema: schema,
	}
}

// Run shows available keys from .env.example and schema, prompts the user for
// values, and creates the envFile. Returns nil on success or an error if the
// user cancels.
func (o *Onboarder) Run(envFile, exampleFile string) error {
	scanner := bufio.NewScanner(o.r)

	// Gather keys from .env.example and schema.
	keys, examples := o.gatherKeys(exampleFile)

	// Welcome message.
	fmt.Fprintln(o.w, "")
	fmt.Fprintln(o.w, "Welcome to Stackup! It looks like you don't have a .env file yet.")
	fmt.Fprintln(o.w, "")
	if len(keys) > 0 {
		fmt.Fprintln(o.w, "The following environment variables are needed:")
		for _, k := range keys {
			line := "  " + k
			if examples[k] != "" {
				line += fmt.Sprintf(" (example: %s)", examples[k])
			}
			if sv, ok := o.schema[k]; ok && sv.Default != "" {
				line += fmt.Sprintf(" [default: %s]", sv.Default)
			}
			fmt.Fprintln(o.w, line)
		}
		fmt.Fprintln(o.w, "")
	}

	// Confirm.
	fmt.Fprint(o.w, "Create your .env now? [Y/n] ")
	if !scanner.Scan() {
		return fmt.Errorf("onboarding cancelled: no input")
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer != "" && !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		return fmt.Errorf("onboarding cancelled by user")
	}

	// Prompt for each key.
	values := make(map[string]string, len(keys))
	for _, k := range keys {
		prompt := fmt.Sprintf("  %s", k)
		if sv, ok := o.schema[k]; ok && sv.Default != "" {
			prompt += fmt.Sprintf(" [%s]", sv.Default)
		}
		prompt += ": "
		fmt.Fprint(o.w, prompt)

		if !scanner.Scan() {
			return fmt.Errorf("onboarding cancelled: unexpected end of input")
		}
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			if sv, ok := o.schema[k]; ok && sv.Default != "" {
				val = sv.Default
			}
		}
		values[k] = val
	}

	// Write .env file.
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, values[k]))
	}
	if err := os.WriteFile(envFile, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("writing .env: %w", err)
	}

	fmt.Fprintln(o.w, "")
	fmt.Fprintln(o.w, "✓ .env created")
	fmt.Fprintln(o.w, "→ Starting stack...")
	return nil
}

// gatherKeys returns a sorted list of unique keys from the example file and
// schema, plus a map of example values from the example file.
func (o *Onboarder) gatherKeys(exampleFile string) ([]string, map[string]string) {
	examples := make(map[string]string)
	keySet := make(map[string]struct{})

	// Read .env.example if it exists.
	if exampleFile != "" {
		if envMap, err := godotenv.Read(exampleFile); err == nil {
			for k, v := range envMap {
				keySet[k] = struct{}{}
				examples[k] = v
			}
		}
	}

	// Add keys from schema.
	for k := range o.schema {
		keySet[k] = struct{}{}
	}

	// Sort for deterministic output.
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys, examples
}
