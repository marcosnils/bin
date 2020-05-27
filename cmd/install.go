//TODO implement --force flag for install
package cmd

import (
	"fmt"
	"io"
	"net/url"
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
		Short:         "Installs the epecified project",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := args[0]

			//TODO make this path optional
			//TODO validate path is valid
			path := args[1]

			//TODO check if binary already exists in config
			// and triger the update process if that's the case

			purl, err := url.Parse(u)

			if err != nil {
				return err
			}

			p, err := providers.New(purl)
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

			if err = saveToDisk(pResult, path); err != nil {
				return fmt.Errorf("Error installing binary %w", err)
			}

			err = config.AddBinary(&config.Binary{
				Path:    path,
				Version: pResult.Version,
				Hash:    fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
				URL:     purl.String(),
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

//TODO check if other binary has the same hash and warn about it
func saveToDisk(f *providers.File, path string) error {

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0766)

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
