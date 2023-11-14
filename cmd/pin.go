package cmd

import (
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

			for _, b := range cfg.Bins {
				for _, p := range args {
					if b.RemoteName == p {
						bin, err := getBinPath(p)
						if err != nil {
							return err
						}
						updatedCfg := cfg.Bins[bin]
						updatedCfg.Pin = true

						err = config.UpsertBinary(updatedCfg)
						if err != nil {
							return err
						}
					}

					// TODO return error for unmatched ones
				}
			}

			return nil
		},
	}

	root.cmd = cmd
	return root
}
