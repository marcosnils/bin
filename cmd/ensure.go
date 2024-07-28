package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/apex/log"
	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type ensureCmd struct {
	cmd *cobra.Command
}

func newEnsureCmd() *ensureCmd {
	root := &ensureCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "ensure [binary_path]...",
		Aliases:       []string{"e"},
		Short:         "Ensures that all binaries listed in the configuration are present",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// TODO: code smell here, this pretty much does
			// the same thing as install logic. Refactor to
			// use the same code in both places
			for _, binCfg := range binsToProcess {
				ep := os.ExpandEnv(binCfg.Path)
				_, err := os.Stat(ep)

				if err == nil {
					f, err := os.Open(ep)
					if err != nil {
						return err
					}

					h := sha256.New()
					if _, err := io.Copy(h, f); err != nil {
						return err
					}

					if fmt.Sprintf("%x", h.Sum(nil)) == binCfg.Hash {
						continue
					}

					log.Infof("%s hash does not match with config's, re-installing", ep)

				} else if !os.IsNotExist(err) {
					continue
				}

				p, err := providers.New(binCfg.URL, binCfg.Provider, binCfg.LatestURL)
				if err != nil {
					return err
				}

				pResult, err := p.Fetch(&providers.FetchOpts{Version: binCfg.Version})
				if err != nil {
					return err
				}

				hash, err := saveToDisk(pResult, ep, true)
				if err != nil {
					return fmt.Errorf("error installing binary: %w", err)
				}

				err = config.UpsertBinary(&config.Binary{
					RemoteName: pResult.Name,
					Path:       binCfg.Path,
					Version:    pResult.Version,
					Hash:       fmt.Sprintf("%x", hash),
					URL:        binCfg.URL,
					LatestURL:  binCfg.LatestURL,
				})
				if err != nil {
					return err
				}
				log.Infof("Done ensuring %s to %s", os.ExpandEnv(binCfg.Path), color.GreenString(binCfg.Version))
			}
			return nil
		},
	}

	root.cmd = cmd
	return root
}
