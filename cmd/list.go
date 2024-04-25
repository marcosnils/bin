package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

// Pad given string with spaces to the right
func _rPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
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

			// Calculate maximum length of each column
			maxLengths := make([]int, 3)
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
			}

			pL, vL, uL := maxLengths[0], maxLengths[1], maxLengths[2]
			magentaItalic := color.New(color.FgMagenta, color.Italic).Sprint
			p, v, u, s := magentaItalic(_rPad(("Path"), pL)), magentaItalic(_rPad("Version", vL)), magentaItalic(_rPad("URL", uL)), magentaItalic("Status")

			fmt.Printf("\n%s  %s  %s  %s", p, v, u, s)

			for _, k := range binPaths {
				b := cfg.Bins[k]

				p := os.ExpandEnv(b.Path)

				_, err := os.Stat(p)

				status := color.GreenString("OK")
				if err != nil {
					status = color.RedString("missing file")
				}

				fmt.Printf("\n%s  %s  %s  %s", _rPad(p, pL), _rPad(b.Version, vL), _rPad(b.URL, uL), status)
			}
			fmt.Print("\n\n")
			return nil
		},
	}

	root.cmd = cmd
	return root
}
