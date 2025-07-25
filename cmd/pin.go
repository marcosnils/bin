package cmd

import (
	"strings"

	"github.com/caarlos0/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type pinCmd struct {
	cmd *cobra.Command
}

func newPinCmd() *pinCmd {
	root := &pinCmd{}

	cmd := &cobra.Command{
		Use:           "pin [<name> | <paths...>]",
		Short:         "Pins current version of the binaries",
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			binsToPin := map[string]*config.Binary{}

			// To pin
			if len(args) > 0 {
				for _, a := range args {
					bin, err := getBinPath(a)
					if err != nil {
						return err
					}
					binsToPin[a] = cfg.Bins[bin]
				}
			} else {
				return nil
			}

			pinned := []string{}

			// Pinning
			for name, bin := range binsToPin {
				bin.Pinned = true
				err := config.UpsertBinary(bin)
				if err != nil {
					return err
				}
				pinned = append(pinned, name)
				continue
			}

			log.Infof("Pinned " + strings.Join(pinned, " "))

			return nil
		},
	}

	root.cmd = cmd
	return root
}
