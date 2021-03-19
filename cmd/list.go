package cmd

import (
	"fmt"
	"os"

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

			cfg := config.Get()

			fmt.Fprintf(w, "\n %s\t%s\t%s\t%s", "Path", "Version", "URL", "Status")
			for _, b := range cfg.Bins {

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
