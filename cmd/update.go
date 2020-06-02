package cmd

import (
	"fmt"

	"github.com/apex/log"
	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type updateCmd struct {
	cmd  *cobra.Command
	opts updateOpts
}

type updateOpts struct {
}

func newUpdateCmd() *updateCmd {
	var root = &updateCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "update",
		Aliases:       []string{"u"},
		Short:         "Updates binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			//TODO add support o update a single binary with
			//`bin update <binary>`

			//TODO update should check all binaries with a
			//certain configured parallelism (default 10, can be changed with -p) and report
			//which binarines could be potentially upgraded.
			//It's very likely that we have to extend the provider
			//interface to support this use-case

			type updateInfo struct{ version, url string }

			toUpdate := map[updateInfo]*config.Binary{}
			cfg := config.Get()
			for _, b := range cfg.Bins {

				p, err := providers.New(b.URL)

				if err != nil {
					return err
				}

				log.Debugf("Checking updates for %s", b.Path)
				v, u, err := p.GetLatestVersion(b.RemoteName)

				if err != nil {
					return fmt.Errorf("Error checking updates for %s, %w", b.Path, err)
				}

				if b.Version != v {
					log.Debugf("Found new version %s for %s", v, b.Path)
					log.Infof("%s %s -> %s ", b.Path, color.YellowString(b.Version), color.GreenString(v))
					toUpdate[updateInfo{v, u}] = b
				}
			}

			if len(toUpdate) == 0 {
				log.Infof("All binaries are up to date")
				return nil
			}

			//TODO will have to refactor this prompt to a separate function
			//so it can be reused in some other places
			fmt.Printf("\nDo you want to continue? [Y/n] ")
			var response string

			_, err := fmt.Scanln(&response)

			if err != nil {
				return fmt.Errorf("Invalid input")
			}

			if response != "Y" {
				return fmt.Errorf("Update aborted")
			}

			//TODO 	:S code smell here, this pretty much does
			//the same thing as install logic. Refactor to
			//use the same code in both places
			for ui, b := range toUpdate {

				p, err := providers.New(ui.url)

				if err != nil {
					return err
				}

				pResult, err := p.Fetch()

				if err != nil {
					return err
				}

				if err = saveToDisk(pResult, b.Path, true); err != nil {
					return fmt.Errorf("Error installing binary %w", err)
				}

				err = config.UpsertBinary(&config.Binary{
					RemoteName: pResult.Name,
					Path:       b.Path,
					Version:    pResult.Version,
					Hash:       fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
					URL:        ui.url,
				})
				log.Infof("Done updating %s to %s", b.Path, color.GreenString(ui.version))

			}
			return nil
		},
	}

	root.cmd = cmd
	return root
}
