package cmd

import (
	"fmt"
	"strings"

	"github.com/apex/log"
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

			binaryList := make(map[string]*config.Binary)
			for _, b := range cfg.Bins {
				binaryList[b.RemoteName] = b
			}

			for _, p := range args {
				bin, found := binaryList[p]
				if found {
					bin.Pinned = true
					err := config.UpsertBinary(bin)
					if err != nil {
						return err
					}
					continue
				}

				return fmt.Errorf("Binary \"%s\" not found", p)
			}

			log.Infof("Pinned " + strings.Join(args, ", "))
			return nil
		},
	}

	root.cmd = cmd
	return root
}
