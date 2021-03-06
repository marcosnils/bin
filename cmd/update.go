package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

type updateInfo struct{ version, url string }

func newUpdateCmd() *updateCmd {
	var root = &updateCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "update [binary_path]",
		Aliases:       []string{"u"},
		Short:         "Updates one or multiple binaries managed by bin",
		SilenceUsage:  true,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			//TODO add support to update from a specific URL.
			//This allows to update binares from a repo that contains
			//multiple tags for different binaries

			var bin string
			if len(args) > 0 {
				bin = args[0]
			}

			//TODO update should check all binaries with a
			//certain configured parallelism (default 10, can be changed with -p) and report
			//which binarines could be potentially upgraded.
			//It's very likely that we have to extend the provider
			//interface to support this use-case

			toUpdate := map[*updateInfo]*config.Binary{}
			cfg := config.Get()

			// Update single binary
			if bin != "" {
				if !strings.Contains(bin, "/") {
					var err error
					bin, err = getBinPath(bin)
					if err != nil {
						return err
					}
				}
				b := cfg.Bins[bin]

				if ui, err := getLatestVersion(b); err != nil {
					return err
				} else if ui != nil {
					toUpdate[ui] = b
				}

			} else {
				for _, b := range cfg.Bins {
					if ui, err := getLatestVersion(b); err != nil {
						return err
					} else if ui != nil {
						toUpdate[ui] = b
					}
				}
			}

			if len(toUpdate) == 0 {
				log.Infof("All binaries are up to date")
				return nil
			}

			//TODO will have to refactor this prompt to a separate function
			//so it can be reused in some other places
			fmt.Printf("\nDo you want to continue? [Y/n] ")
			reader := bufio.NewReader(os.Stdin)
			var response string

			response, err := reader.ReadString('\n')

			if err != nil {
				return fmt.Errorf("Invalid input")
			}

			switch strings.ToLower(strings.TrimSpace(response)) {
			case "y", "yes":
			default:
				return fmt.Errorf("Command aborted")
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

func getLatestVersion(b *config.Binary) (*updateInfo, error) {
	p, err := providers.New(b.URL)

	if err != nil {
		return nil, err
	}

	log.Debugf("Checking updates for %s", b.Path)
	v, u, err := p.GetLatestVersion()

	if err != nil {
		return nil, fmt.Errorf("Error checking updates for %s, %w", b.Path, err)
	}

	if b.Version == v {
		return nil, nil
	}
	log.Debugf("Found new version %s for %s at %s", v, b.Path, u)
	log.Infof("%s %s -> %s (%s)", b.Path, color.YellowString(b.Version), color.GreenString(v), u)
	return &updateInfo{v, u}, nil
}
