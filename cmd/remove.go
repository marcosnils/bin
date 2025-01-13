package cmd

import (
	"fmt"
	"github.com/apex/log"
	"os"
	"os/exec"

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
					bp, err := getBinPath(p)
					if err != nil {
						return err
					}
					if os.ExpandEnv(b.Path) == os.ExpandEnv(bp) {
						err := os.Remove(os.ExpandEnv(bp))
						existingToRemove = append(existingToRemove, b.Path)
						// TODO some providers (like docker) might download
						// additional things somewhere else, maybe we should
						// call the provider to do a cleanup here.
						if err != nil {
							return fmt.Errorf("Error removing path %s: %v", os.ExpandEnv(bp), err)
						}
						continue

					}
				}
			}
			hooks := config.GetHooks(config.PreRemove)
			for _, hook := range hooks {
				if hook.Command != "" {
					log.Infof("Executing pre-remove hook: %s %v", hook.Command, hook.Args)
					output, err := exec.Command(hook.Command, hook.Args...).CombinedOutput()
					if err != nil {
						log.Errorf("Error executing hook: %s, output: %s, error: %v", hook.Command, string(output), err)
						return err
					}
					log.Infof("Hook executed successfully: %s", string(output))
				}
			}
			err := config.RemoveBinaries(existingToRemove)
			if err != nil {
				return err
			}
			hooks = config.GetHooks(config.PostRemove)
			for _, hook := range hooks {
				if hook.Command != "" {
					log.Infof("Executing psot-remove hook: %s %v", hook.Command, hook.Args)
					output, err := exec.Command(hook.Command, hook.Args...).CombinedOutput()
					if err != nil {
						log.Errorf("Error executing hook: %s, output: %s, error: %v", hook.Command, string(output), err)
						return err
					}
					log.Infof("Hook executed successfully: %s", string(output))
				}
			}
			return nil
		},
	}

	root.cmd = cmd
	return root
}
