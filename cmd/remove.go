package cmd

import (
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type removeCmd struct {
	cmd  *cobra.Command
	opts removeOpts
}

type removeOpts struct {
}

func newRemoveCmd() *removeCmd {
	var root = &removeCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "remove [<name> | <paths...>]",
		Aliases:       []string{"rm"},
		Short:         "Removes binaries managed by bin",
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			cfg := config.Get()

			existingToRemove := []string{}

			for _, p := range args {
				p, err := getBinPath(p)
				if err != nil {
					return err
				}

				if _, ok := cfg.Bins[p]; ok {
					existingToRemove = append(existingToRemove, p)
					//TODO some providers (like docker) might download
					//additional things somewhere else, maybe we should
					//call the provider to do a cleanup here.
					err := os.Remove(p)
					if err != nil {
						return fmt.Errorf("Error removing path %s: %v", p, err)
					}
					continue
				}
				log.Infof("Path %s not found in bin, ignoring.", p)
			}

			config.RemoveBinaries(existingToRemove)
			return nil
		},
	}

	root.cmd = cmd
	return root
}
