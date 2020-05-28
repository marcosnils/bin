package cmd

import (
	"os"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
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
			cfg := config.Get()

			pathsToDel := []string{}
			for _, b := range cfg.Bins {
				if _, err := os.Stat(b.Path); os.IsNotExist(err) {
					log.Infof("%s not found removing", b.Path)
					pathsToDel = append(pathsToDel, b.Path)
				}
			}

			return config.RemoveBinaries(pathsToDel)
		},
	}

	root.cmd = cmd
	return root
}
