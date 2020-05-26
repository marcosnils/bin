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
		Short:         "Installs the epecified project",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			//TODO make this path optional
			path := args[1]

			//TODO check if binary already exists in config
			// and triger the update process if that's the case

			p, err := providers.New(url)
			if err != nil {
				return err
			}

			pResult, err := p.Fetch()

			if err != nil {
				return err
			}

			if err = saveToDisk(pResult, path); err != nil {
				return fmt.Errorf("Error installing binary %w", err)
			}

			err = config.AddBinary(&config.Binary{
				Path:    filepath.Join(path, pResult.Name),
				Version: pResult.Version,
				Hash:    fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
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

// checkPath checks that path exists and is a directory
// it should additionally check if there's already a binary with the same
// file and ask the user if he wants to override
func checkPath(path string) error {
	return nil
}

// saveToDisk saves the specified binary to the desired path
// and makes it executable
func saveToDisk(f *providers.File, path string) error {

	//TODO implement checkpath
	err := checkPath(path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(path, f.Name), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0766)

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
