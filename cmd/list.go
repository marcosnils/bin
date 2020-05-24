package cmd

import (
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
			return nil
		},
	}

	root.cmd = cmd
	return root
}
