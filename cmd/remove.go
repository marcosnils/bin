package cmd

import (
	"github.com/spf13/cobra"
)

type removeCmd struct {
	cmd  *cobra.Command
	opts removeOpts
}

type removeOpts struct {
}

func newRemoveCmd() *removeCmd {
	var root = &removeCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "remove",
		Aliases:       []string{"r"},
		Short:         "Removes binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	root.cmd = cmd
	return root
}
