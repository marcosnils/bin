package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

// Pad given string with spaces to the right
func _rPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// cooldownCell renders the Cooldown column value for a binary:
//   - "-"   when the effective cooldown is disabled (0)
//   - "n/a" when the provider can't supply a publish date (not enforceable)
//   - "<dur>" otherwise, with a trailing "*" for an explicit per-package override
func cooldownCell(b *config.Binary) string {
	eff, err := config.EffectiveCooldown(b)
	if err != nil || eff <= 0 {
		return "-"
	}
	if !providers.SupportsPublishDate(b.Provider) {
		return "n/a"
	}
	s := config.FormatCooldown(eff)
	if b.Cooldown != "" {
		s += "*"
	}
	return s
}

type listCmd struct {
	cmd *cobra.Command
}

func newListCmd() *listCmd {
	root := &listCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "list",
		Aliases:       []string{"ls"},
		Short:         "List binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			binPaths := []string{}
			for k := range cfg.Bins {
				binPaths = append(binPaths, k)
			}
			sort.Strings(binPaths)

			// Pre-compute the cooldown cell for each binary.
			cooldowns := map[string]string{}
			for _, k := range binPaths {
				cooldowns[k] = cooldownCell(cfg.Bins[k])
			}

			// Calculate maximum length of each column
			maxLengths := make([]int, 4)
			for _, k := range binPaths {
				b := cfg.Bins[k]
				p := os.ExpandEnv(b.Path)

				if len(p) > maxLengths[0] {
					maxLengths[0] = len(p)
				}

				if len(b.Version) > maxLengths[1] {
					maxLengths[1] = len(b.Version)
				}

				if len(b.URL) > maxLengths[2] {
					maxLengths[2] = len(b.URL)
				}

				if len(cooldowns[k]) > maxLengths[3] {
					maxLengths[3] = len(cooldowns[k])
				}
			}

			pL, vL, uL, cL := maxLengths[0], maxLengths[1], maxLengths[2], maxLengths[3]
			// Reserve an extra column for the pin marker so that pinned and
			// unpinned versions line up.
			vL++
			if cL < len("Cooldown") {
				cL = len("Cooldown")
			}
			magentaItalic := color.New(color.FgMagenta, color.Italic).Sprint
			// Leading space keeps the header aligned with the version text
			// rather than the pin-marker column.
			p, v, u := magentaItalic(_rPad(("Path"), pL)), magentaItalic(_rPad(" Version", vL)), magentaItalic(_rPad("URL", uL))
			c, s := magentaItalic(_rPad("Cooldown", cL)), magentaItalic("Status")

			fmt.Printf("\n%s  %s  %s  %s  %s", p, v, u, c, s)

			for _, k := range binPaths {
				b := cfg.Bins[k]

				p := os.ExpandEnv(b.Path)

				_, err := os.Stat(p)

				status := color.GreenString("OK")
				if err != nil {
					status = color.RedString("missing %s", p)
				}

				marker := " "
				if b.Pinned {
					marker = "*"
				}

				fmt.Printf("\n%s  %s  %s  %s  %s", _rPad(p, pL), _rPad(marker+b.Version, vL), _rPad(b.URL, uL), _rPad(cooldowns[k], cL), status)
			}
			fmt.Print("\n\n")
			// Legend: distinguish the pin marker from the cooldown-override marker.
			fmt.Println("Version *: pinned   Cooldown *: per-package override   n/a: provider has no publish date")
			return nil
		},
	}

	root.cmd = cmd
	return root
}
