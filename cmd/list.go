package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type listCmd struct {
	cmd  *cobra.Command
	opts listOpts
}

type listOpts struct {
}

func newListCmd() *listCmd {
	var root = &listCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "list",
		Aliases:       []string{"ls"},
		Short:         "List binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := new(tabwriter.Writer)
			w.Init(os.Stdout, 8, 8, 2, '\t', 0)

			defer w.Flush()

			cfg := config.Get()

			fmt.Fprintf(w, "\n %s\t%s\t%s\t", "Path", "Version", "URL")
			for _, b := range cfg.Bins {
				fmt.Fprintf(w, "\n %s\t%s\t%s\t", b.Path, b.Version, b.URL)
			}
			fmt.Fprintf(w, "\n")
			return nil
		},
	}

	root.cmd = cmd
	return root
}
