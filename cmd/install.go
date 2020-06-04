package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/spf13/cobra"
)

type installCmd struct {
	cmd  *cobra.Command
	opts installOpts
}

type installOpts struct {
}

func newInstallCmd() *installCmd {
	var root = &installCmd{}
	// nolint: dupl
	var cmd = &cobra.Command{
		Use:           "install",
		Aliases:       []string{"i"},
		Short:         "Installs the specified project",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			//TODO implement --force(-f) flag for install
			// to override the binary if exists
			u := args[0]

			//TODO make this path optional. If the path
			//is not specified bin could automatically
			//select a PATH that could write and install the binaries there.
			//Additionally, it could store that path in the config file so it doesn't
			//have to calculate it each time. Afterwards, bin users can change this
			//path by editing bin's config file or maybe introdice the `bin config` command

			var path string
			if len(args) > 1 {
				path = args[1]
			} else if len(config.Get().DefaultPath) > 0 {
				path = config.Get().DefaultPath
			} else {
				var err error
				path, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			//TODO check if binary already exists in config
			// and triger the update process if that's the case

			p, err := providers.New(u)
			if err != nil {
				return err
			}

			pResult, err := p.Fetch()

			if err != nil {
				return err
			}

			path, err = getFinalPath(path, pResult.Name)

			if err != nil {
				return err
			}

			if err = saveToDisk(pResult, path, false); err != nil {
				return fmt.Errorf("Error installing binary %w", err)
			}

			err = config.UpsertBinary(&config.Binary{
				RemoteName: pResult.Name,
				Path:       path,
				Version:    pResult.Version,
				Hash:       fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
				URL:        u,
			})

			if err != nil {
				return err
			}

			log.Infof("Done installing %s %s", pResult.Name, pResult.Version)

			return nil
		},
	}

	root.cmd = cmd
	return root
}

// getFinalPath checks if path exists and returns
// true if it's a directory. If false, it also
// checks if the path already exists and prompts
// the user to override
func getFinalPath(path, fileName string) (string, error) {
	fi, err := os.Stat(path)

	//TODO implement file existence and override logic
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

//TODO check if other binary has the same hash and warn about it.
//TODO if the file is zipped, tared, whatever then extract it
func saveToDisk(f *providers.File, path string, overwrite bool) error {

	var extraFlags int = os.O_EXCL

	if overwrite {
		extraFlags = 0
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|extraFlags, 0766)

	if err != nil {
		return err
	}

	defer file.Close()
	defer f.Data.Close()

	//TODO add a spinner here indicating that the binary is being downloaded
	log.Infof("Starting download for %s@%s into %s", f.Name, f.Version, path)
	_, err = io.Copy(file, f.Data)
	if err != nil {
		return err
	}

	return nil
}
