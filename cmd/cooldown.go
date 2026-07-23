package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/caarlos0/log"
	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type cooldownCmd struct {
	cmd *cobra.Command
}

// newCooldownCmd builds the `bin cooldown` command tree. A cooldown holds back
// updating a binary until the configured period has elapsed since the target
// version was published, mitigating supply-chain attacks that rely on quick
// adoption of a freshly published (malicious) release.
func newCooldownCmd() *cooldownCmd {
	root := &cooldownCmd{}

	cmd := &cobra.Command{
		Use:   "cooldown",
		Short: "Manage update cooldown periods",
		Long: `Manage update cooldown periods.

A cooldown delays updating a binary until the given duration has elapsed since
the new version was published, giving the community time to detect malicious
releases. Durations accept a friendly format: e.g. 24h, 90m, 7d, 2w. Set 0 to
disable. Cooldowns are opt-in and default to disabled.

Cooldowns can only be enforced for providers that expose a publish date
(GitHub, GitLab, Codeberg, goinstall); they are ignored for others.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCooldownList()
		},
	}

	cmd.AddCommand(
		newCooldownListCmd(),
		newCooldownGetCmd(),
		newCooldownSetCmd(),
		newCooldownUnsetCmd(),
	)

	root.cmd = cmd
	return root
}

func newCooldownListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Aliases:       []string{"ls"},
		Short:         "Show the default cooldown and any per-package overrides",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCooldownList()
		},
	}
}

func newCooldownGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "get [binary]",
		Short:         "Show the effective cooldown for the default or a binary",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				d, err := config.EffectiveCooldown(nil)
				if err != nil {
					return err
				}
				log.Infof("default cooldown: %s", config.FormatCooldown(d))
				return nil
			}

			bin, err := getBinPath(args[0])
			if err != nil {
				return err
			}
			b := config.Get().Bins[bin]
			eff, err := config.EffectiveCooldown(b)
			if err != nil {
				return err
			}

			source := "default"
			if b.Cooldown != "" {
				source = "override"
			}
			log.Infof("%s cooldown: %s (%s)", bin, config.FormatCooldown(eff), source)
			if !providers.SupportsPublishDate(b.Provider) {
				log.Warnf("provider %q has no publish date; cooldown is not enforced for this binary", b.Provider)
			}
			return nil
		},
	}
}

func newCooldownSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [binary] <duration>",
		Short: "Set the default cooldown, or a per-package override",
		Long: `Set the default cooldown, or a per-package override.

With a single argument the global default is set:
    bin cooldown set 24h

With two arguments a per-package override is set:
    bin cooldown set fzf 7d

Use 0 to disable.`,
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				// Global default. Validate before persisting so a mistyped
				// binary name isn't silently stored as the default.
				if _, err := config.ParseCooldown(args[0]); err != nil {
					return fmt.Errorf("%w (to set a per-package cooldown use: bin cooldown set <binary> <duration>)", err)
				}
				if err := config.SetDefaultCooldown(args[0]); err != nil {
					return err
				}
				log.Infof("default cooldown set to %s", config.FormatCooldown(mustParse(args[0])))
				return nil
			}

			// Per-package override.
			if _, err := config.ParseCooldown(args[1]); err != nil {
				return err
			}
			bin, err := getBinPath(args[0])
			if err != nil {
				return err
			}
			b := config.Get().Bins[bin]
			b.Cooldown = args[1]
			if err := config.UpsertBinary(b); err != nil {
				return err
			}
			log.Infof("cooldown for %s set to %s", bin, config.FormatCooldown(mustParse(args[1])))
			if !providers.SupportsPublishDate(b.Provider) {
				log.Warnf("provider %q has no publish date; cooldown is not enforced for this binary", b.Provider)
			}
			return nil
		},
	}
}

func newCooldownUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "unset [binary]",
		Short:         "Clear the default cooldown, or a per-package override",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := config.SetDefaultCooldown(""); err != nil {
					return err
				}
				log.Infof("default cooldown cleared")
				return nil
			}

			bin, err := getBinPath(args[0])
			if err != nil {
				return err
			}
			b := config.Get().Bins[bin]
			b.Cooldown = ""
			if err := config.UpsertBinary(b); err != nil {
				return err
			}
			log.Infof("cooldown override for %s cleared", bin)
			return nil
		},
	}
}

// runCooldownList prints the resolved default cooldown followed by any binaries
// carrying an explicit per-package override.
func runCooldownList() error {
	cfg := config.Get()

	def, err := config.EffectiveCooldown(nil)
	if err != nil {
		return err
	}
	fmt.Printf("Default cooldown: %s\n", config.FormatCooldown(def))

	overrides := []string{}
	for k, b := range cfg.Bins {
		if b.Cooldown != "" {
			overrides = append(overrides, k)
		}
	}
	sort.Strings(overrides)

	if len(overrides) == 0 {
		fmt.Println("No per-package overrides.")
		return nil
	}

	fmt.Println("\nPer-package overrides:")
	for _, k := range overrides {
		b := cfg.Bins[k]
		eff, err := config.EffectiveCooldown(b)
		if err != nil {
			return err
		}
		note := ""
		if !providers.SupportsPublishDate(b.Provider) {
			note = color.YellowString("  (not enforced: provider has no publish date)")
		}
		fmt.Printf("  %s  %s%s\n", os.ExpandEnv(b.Path), config.FormatCooldown(eff), note)
	}
	return nil
}

// mustParse parses an already-validated cooldown string for display purposes.
func mustParse(s string) (d time.Duration) {
	d, _ = config.ParseCooldown(s)
	return d
}
