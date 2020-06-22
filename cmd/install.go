package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/options"
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
	defer f.Data.Close()

	var buf bytes.Buffer
	tee := io.TeeReader(f.Data, &buf)

	t, err := filetype.MatchReader(tee)
	if err != nil {
		return err
	}

	var outputFile = io.MultiReader(&buf, f.Data)

	if t == matchers.TypeGz {
		fileName, file, err := processTarGz(outputFile)
		if err != nil {
			return err
		}
		outputFile = file
		path = strings.Replace(path, filepath.Base(path), fileName, -1)

	}

	var extraFlags int = os.O_EXCL

	if overwrite {
		extraFlags = 0
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|extraFlags, 0766)

	if err != nil {
		return err
	}

	defer file.Close()

	//TODO add a spinner here indicating that the binary is being downloaded
	log.Infof("Starting download for %s@%s into %s", f.Name, f.Version, path)
	_, err = io.Copy(file, outputFile)
	if err != nil {
		return err
	}

	return nil
}

// processTar receives a tar.gz file and returns the
// correct file for bin to download
func processTarGz(r io.Reader) (string, io.Reader, error) {
	// We're caching the whole file into memory so we can prompt
	// the user which file they want to download

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	br := bytes.NewReader(b)

	gr, err := gzip.NewReader(br)
	if err != nil {
		return "", nil, err
	}

	tr := tar.NewReader(gr)
	tarFiles := []interface{}{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		if header.Typeflag == tar.TypeReg {
			tarFiles = append(tarFiles, header.Name)
		}
	}
	if len(tarFiles) == 0 {
		return "", nil, errors.New("No files found in tar archive")
	}

	selectedFile := options.Select("Select file to download:", tarFiles).(string)

	// Reset readers so we can scan the tar file
	// again to get the correct file reader
	br.Seek(0, io.SeekStart)
	gr, err = gzip.NewReader(br)
	if err != nil {
		return "", nil, err
	}
	tr = tar.NewReader(gr)

	var fr io.Reader
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}

		if header.Name == selectedFile {
			fr = tr
			break
		}
	}
	// return base of selected file since tar
	// files usually have folders inside
	return filepath.Base(selectedFile), fr, nil

}
