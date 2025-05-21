package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type removeCmd struct {
	cmd *cobra.Command
}

func newRemoveCmd() *removeCmd {
	root := &removeCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "remove [<name> | <paths...>]",
		Aliases:       []string{"rm"},
		Short:         "Removes binaries managed by bin",
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Get()

			existingToRemove := []string{}

			for _, b := range cfg.Bins {
				for _, p := range args {
					// TODO: avoid calling getBinPath each time and make it
					// once at the beginning for each arg
					bp, err := getBinPath(p)

					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return err
					}
					if os.ExpandEnv(b.Path) == os.ExpandEnv(bp) || p == b.Path {
						existingToRemove = append(existingToRemove, b.Path)

						// TODO some providers (like docker) might download
						// additional things somewhere else, maybe we should
						// call the provider to do a cleanup here.
						if err := os.Remove(os.ExpandEnv(bp)); err != nil && !os.IsNotExist(err) {
							return fmt.Errorf("Error removing path %s: %v", os.ExpandEnv(bp), err)
						}
						continue
					}
				}
			}
			err := config.RemoveBinaries(existingToRemove)
			return err
		},
	}

	root.cmd = cmd
	return root
}
