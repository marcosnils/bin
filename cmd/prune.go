package cmd

import (
	"os"

	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/prompt"
	"github.com/spf13/cobra"
)

type pruneCmd struct {
	cmd  *cobra.Command
	opts pruneOpts
}

type pruneOpts struct {
	force bool
}

func newPruneCmd() *pruneCmd {
	root := &pruneCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "prune",
		Short:         "Prunes binaries that no longer exist in the system",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			pathsToDel := []string{}
			for _, b := range cfg.Bins {
				ep := os.ExpandEnv(b.Path)
				if _, err := os.Stat(ep); os.IsNotExist(err) {
					log.Infof("%s not found removing", ep)
					pathsToDel = append(pathsToDel, b.Path)
				}
			}

			if len(pathsToDel) == 0 {
				return nil
			}

			if !root.opts.force {
				err := prompt.Confirm("The following paths will be removed. Continue?")
				if err != nil {
					return err
				}
			}

			return config.RemoveBinaries(pathsToDel)
		},
	}

	root.cmd = cmd
	root.cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, "Bypass confirmation prompt")
	return root
}
