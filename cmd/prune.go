package cmd

import (
	"github.com/spf13/cobra"
)

type pruneCmd struct {
	cmd  *cobra.Command
	opts pruneOpts
}

type pruneOpts struct {
}

func newPruneCmd() *pruneCmd {
	var root = &pruneCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "prune",
		Short:         "Prunes binaries that no longer exist in the system",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			//TODO implement prune
			return nil
		},
	}

	root.cmd = cmd
	return root
}
