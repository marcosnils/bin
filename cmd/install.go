package cmd

import (
	"github.com/spf13/cobra"
)

type installCmd struct {
	cmd  *cobra.Command
	opts installOpts
}

type installOpts struct {
}

func newInstallCmd() *installCmd {
	var root = &installCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "install",
		Aliases:       []string{"i"},
		Short:         "Installs the specified project",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	root.cmd = cmd
	return root
}
