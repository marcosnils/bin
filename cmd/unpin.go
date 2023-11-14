package cmd

import (
	"fmt"
	"strings"

	"github.com/apex/log"
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

			binaryList := make(map[string]*config.Binary)
			for _, b := range cfg.Bins {
				binaryList[b.RemoteName] = b
			}

			for _, p := range args {
				bin, found := binaryList[p]
				if found {
					bin.Pin = false
					err := config.UpsertBinary(bin)
					if err != nil {
						return err
					}
					continue
				}

				return fmt.Errorf("Binary \"%s\" not found", p)
			}

			log.Infof("Unpinned " + strings.Join(args, ", "))
			return nil
		},
	}

	root.cmd = cmd
	return root
}
