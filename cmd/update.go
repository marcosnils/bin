package cmd

import (
	"github.com/spf13/cobra"
)

type updateCmd struct {
	cmd  *cobra.Command
	opts updateOpts
}

type updateOpts struct {
}

func newUpdateCmd() *updateCmd {
	var root = &updateCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "update",
		Aliases:       []string{"u"},
		Short:         "Updates binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	root.cmd = cmd
	return root
}
