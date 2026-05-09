package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for stackup.

To load completions:

Bash:
  $ source <(stackup completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ stackup completion bash > /etc/bash_completion.d/stackup
  # macOS:
  $ stackup completion bash > $(brew --prefix)/etc/bash_completion.d/stackup

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ stackup completion zsh > "${fpath[1]}/_stackup"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ stackup completion fish | source

  # To load completions for each session, execute once:
  $ stackup completion fish > ~/.config/fish/completions/stackup.fish

PowerShell:
  PS> stackup completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add the output to your profile:
  PS> stackup completion powershell >> $PROFILE
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
