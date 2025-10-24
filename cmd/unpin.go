package cmd

import (
	"strings"

	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type unpinCmd struct {
	cmd *cobra.Command
}

func newUnpinCmd() *unpinCmd {
	root := &unpinCmd{}

	cmd := &cobra.Command{
		Use:           "unpin [<name> | <paths...>]",
		Short:         "Unpins current version of the binaries",
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			binsToUnpin := map[string]*config.Binary{}

			// To unpin
			if len(args) > 0 {
				for _, a := range args {
					bin, err := getBinPath(a)
					if err != nil {
						return err
					}
					binsToUnpin[a] = cfg.Bins[bin]
				}
			} else {
				return nil
			}

			unpinned := []string{}

			// Unpinning
			for name, bin := range binsToUnpin {
				bin.Pinned = false
				err := config.UpsertBinary(bin)
				if err != nil {
					return err
				}
				unpinned = append(unpinned, name)
				continue
			}

			log.Infof("Unpinned " + strings.Join(unpinned, " "))

			return nil
		},
	}

	root.cmd = cmd
	return root
}
