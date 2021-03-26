package cmd

import (
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type pruneCmd struct {
	cmd *cobra.Command
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
			_, bins := config.Get()

			pathsToDel := []string{}
			for _, b := range bins {
				if _, err := os.Stat(b.Path); os.IsNotExist(err) {
					log.Infof("%s not found removing", b.Path)
					pathsToDel = append(pathsToDel, b.Path)
				}
			}

			// TODO will have to refactor this prompt to a separate function
			// so it can be reused in some other places
			// TODO add force flag to bypass prompt
			fmt.Printf("\nThe following paths will be removed. Continue? [Y/n] ")
			var response string

			_, err := fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("Invalid input")
			}

			if response != "Y" {
				return fmt.Errorf("Command aborted")
			}

			return config.RemoveBinaries(pathsToDel)
		},
	}

	root.cmd = cmd
	return root
}
