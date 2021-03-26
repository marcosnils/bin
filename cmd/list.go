package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/WeiZhang555/tabwriter"
	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

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
			w := new(tabwriter.Writer)
			w.Init(os.Stdout, 8, 8, 3, '\t', 0)

			defer w.Flush()

			_, bins := config.Get()

			fmt.Fprintf(w, "\n %s\t%s\t%s\t%s", "Path", "Version", "URL", "Status")
			binPaths := []string{}
			for k := range bins {
				binPaths = append(binPaths, k)
			}
			sort.Strings(binPaths)
			for _, k := range binPaths {
				b := bins[k]

				_, err := os.Stat(b.Path)

				status := color.GreenString("OK")
				if err != nil {
					status = color.RedString("missing file")
				}

				fmt.Fprintf(w, "\n %s\t%s\t%s\t%s", b.Path, b.Version, b.URL, status)
			}
			fmt.Fprintf(w, "\n\n")
			return nil
		},
	}

	root.cmd = cmd
	return root
}
