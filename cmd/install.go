package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/assets"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type installCmd struct {
	cmd  *cobra.Command
	opts installOpts
}

type installOpts struct {
	force      bool
	provider   string
	all        bool
	autoSelect string
}

func newInstallCmd() *installCmd {
	root := &installCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "install <url>",
		Aliases:       []string{"i"},
		Short:         "Installs the specified project from a url",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := args[0]

			var resolvedPath string
			if len(args) > 1 {
				resolvedPath = args[1]
			} else if len(config.Get().DefaultPath) > 0 {
				resolvedPath = config.Get().DefaultPath
			} else {
				var err error
				resolvedPath, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			// TODO check if binary already exists in config
			// and triger the update process if that's the case

			p, err := providers.New(u, root.opts.provider)
			if err != nil {
				return err
			}

			pResult, err := p.Fetch(&providers.FetchOpts{All: root.opts.all, AutoSelect: root.opts.autoSelect})
			if err != nil {
				return err
			}

			resolvedPath, err = checkFinalPath(resolvedPath, assets.SanitizeName(pResult.Name, pResult.Version))
			if err != nil {
				return err
			}

			if err = saveToDisk(pResult, resolvedPath, root.opts.force); err != nil {
				return fmt.Errorf("error installing binary: %w", err)
			}

			err = config.UpsertBinary(&config.Binary{
				RemoteName:  pResult.Name,
				Path:        resolvedPath,
				Version:     pResult.Version,
				Hash:        fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
				URL:         u,
				Provider:    p.GetID(),
				PackagePath: pResult.PackagePath,
			})
			if err != nil {
				return err
			}

			log.Infof("Done installing %s %s", pResult.Name, pResult.Version)

			return nil
		},
	}

	root.cmd = cmd
	root.cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, "Force the installation even if the file already exists")
	root.cmd.Flags().BoolVarP(&root.opts.all, "all", "a", false, "Show all possible download options (skip scoring & filtering)")
	root.cmd.Flags().StringVarP(&root.opts.provider, "provider", "p", "", "Forces to use a specific provider")
	root.cmd.Flags().StringVarP(&root.opts.autoSelect, "select", "s", "", "auto select installation file")
	return root
}

// checkFinalPath checks if path exists and if it's a dir or not
// and returns the correct final file path. It also
// checks if the path already exists and prompts
// the user to override
func checkFinalPath(path, fileName string) (string, error) {
	fi, err := os.Stat(os.ExpandEnv(path))

	// TODO implement file existence and override logic
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if fi != nil && fi.IsDir() {
		return filepath.Join(path, fileName), nil
	}

	return path, nil
}

// saveToDisk saves the specified binary to the desired path
// and makes it executable. It also checks if any other binary
// has the same hash and exists if so.

// TODO check if other binary has the same hash and warn about it.
// TODO if the file is zipped, tared, whatever then extract it
func saveToDisk(f *providers.File, path string, overwrite bool) error {
	epath := os.ExpandEnv((path))

	var extraFlags int = os.O_EXCL

	if overwrite {
		extraFlags = 0
		err := os.Remove(epath)
		log.Debugf("Overwrite flag set, removing file %s\n", epath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	file, err := os.OpenFile(epath, os.O_RDWR|os.O_CREATE|extraFlags, 0o766)
	if err != nil {
		return err
	}

	defer file.Close()

	log.Infof("Copying for %s@%s into %s", f.Name, f.Version, epath)
	_, err = io.Copy(file, f.Data)
	if err != nil {
		return err
	}

	return nil
}
