package cmd

import (
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/prompt"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type updateCmd struct {
	cmd  *cobra.Command
	opts updateOpts
}

type updateOpts struct {
	yesToUpdate     bool
	dryRun          bool
	all             bool
	skipPathCheck   bool
	continueOnError bool
}

type updateInfo struct{ version, url string }

func newUpdateCmd() *updateCmd {
	root := &updateCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "update [binary_path]",
		Aliases:       []string{"u"},
		Short:         "Updates one or multiple binaries managed by bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO add support to update from a specific URL.
			// This allows to update binares from a repo that contains
			// multiple tags for different binaries

			// TODO update should check all binaries with a
			// certain configured parallelism (default 10, can be changed with -p) and report
			// which binarines could be potentially upgraded.
			// It's very likely that we have to extend the provider
			// interface to support this use-case

			toUpdate := map[*updateInfo]*config.Binary{}
			cfg := config.Get()
			binsToProcess := map[string]*config.Binary{}

			// Update specific binaries
			if len(args) > 0 {
				for _, a := range args {
					bin, err := getBinPath(a)
					if err != nil {
						return err
					}
					binsToProcess[bin] = cfg.Bins[bin]
				}
			} else {
				binsToProcess = cfg.Bins
			}

			updateFailures := map[*config.Binary]error{}

			for _, b := range binsToProcess {
				p, err := providers.New(b.URL, b.Provider)
				if err != nil {
					return err
				}
				if ui, err := getLatestVersion(b, p); err != nil {
					if root.opts.continueOnError {
						updateFailures[b] = fmt.Errorf("Error while getting latest version of %v: %v", b.Path, err)
						continue
					}
					return err
				} else if ui != nil {
					toUpdate[ui] = b
				}
			}

			if len(toUpdate) == 0 && len(updateFailures) == 0 {
				log.Infof("All binaries are up to date")
				return nil
			}

			if root.opts.dryRun {
				return wrapErrorWithCode(fmt.Errorf("Updates found, exit (dry-run mode)."), 3, "")
			}

			if len(toUpdate) > 0 && !root.opts.yesToUpdate {
				for _, err := range updateFailures {
					log.Warnf("%v", err)
				}
				updateFailures = map[*config.Binary]error{}

				err := prompt.Confirm("Do you want to continue?")
				if err != nil {
					return err
				}
			}

			// TODO	:S code smell here, this pretty much does
			// the same thing as install logic. Refactor to
			// use the same code in both places
			for ui, b := range toUpdate {

				p, err := providers.New(ui.url, b.Provider)
				if err != nil {
					return err
				}

				pResult, err := p.Fetch(&providers.FetchOpts{All: root.opts.all, PackagePath: b.PackagePath, SkipPatchCheck: root.opts.skipPathCheck, PackageName: b.RemoteName, LocalPath: b.Path})
				if err != nil {
					if root.opts.continueOnError {
						updateFailures[b] = fmt.Errorf("Error while fetching %v: %w", ui.url, err)
						continue
					}
					return err
				}

				hash, err := saveToDisk(pResult, b.Path, true)
				if err != nil {
					return fmt.Errorf("error installing binary: %w", err)
				}

				err = config.UpsertBinary(&config.Binary{
					RemoteName:  pResult.Name,
					Path:        b.Path,
					Version:     pResult.Version,
					Hash:        fmt.Sprintf("%x", hash),
					URL:         ui.url,
					PackagePath: pResult.PackagePath,
				})
				if err != nil {
					return err
				}

				log.Infof("Done updating %s to %s", os.ExpandEnv(b.Path), color.GreenString(ui.version))
			}
			for _, err := range updateFailures {
				log.Warnf("%v", err)
			}
			// TODO: Return wrapping error with specific exit code if len(updateFailures) > 0?
			return nil
		},
	}

	root.cmd = cmd
	root.cmd.Flags().BoolVarP(&root.opts.dryRun, "dry-run", "", false, "Only show status, don't prompt for update")
	root.cmd.Flags().BoolVarP(&root.opts.yesToUpdate, "yes", "y", false, "Assume yes to update prompt")
	root.cmd.Flags().BoolVarP(&root.opts.all, "all", "a", false, "Show all possible download options (skip scoring & filtering)")
	root.cmd.Flags().BoolVarP(&root.opts.skipPathCheck, "skip-path-check", "p", false, "Skips path checking when looking into packages")
	root.cmd.Flags().BoolVarP(&root.opts.continueOnError, "continue-on-error", "c", false, "Continues to update next package if an error is encountered")
	return root
}

func getLatestVersion(b *config.Binary, p providers.Provider) (*updateInfo, error) {
	log.Debugf("Checking updates for %s", b.Path)
	v, u, err := p.GetLatestVersion()
	if err != nil {
		return nil, fmt.Errorf("Error checking updates for %s, %w", b.Path, err)
	}

	if b.Version == v {
		return nil, nil
	}

	bSemver, bSemverErr := version.NewVersion(b.Version)
	vSemver, vSemverErr := version.NewVersion(v)
	if bSemverErr == nil && vSemverErr == nil && vSemver.LessThanOrEqual(bSemver) {
		return nil, nil
	}

	log.Debugf("Found new version %s for %s at %s", v, b.Path, u)
	log.Infof("%s %s -> %s (%s)", b.Path, color.YellowString(b.Version), color.GreenString(v), u)
	return &updateInfo{v, u}, nil
}
