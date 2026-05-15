package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/log"
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
	force    bool
	provider string
	all      bool
	name     string
}

func newInstallCmd() *installCmd {
	root := &installCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "install <url> [name | path]",
		Aliases:       []string{"i"},
		Short:         "Installs the specified binary from a url",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := args[0]
			defaultPath := config.Get().DefaultPath

			var resolvedPath string
			if len(args) > 1 {
				resolvedPath = args[1]
				if !strings.Contains(resolvedPath, "/") {
					resolvedPath = filepath.Join(defaultPath, resolvedPath)
				}

			} else {
				resolvedPath = defaultPath
			}

			// TODO check if binary already exists in config
			// and triger the update process if that's the case

			p, err := providers.New(u, root.opts.provider)
			if err != nil {
				return err
			}
			log.Debugf("Using provider '%s' for '%s'", p.GetID(), u)

			pResult, err := p.Fetch(&providers.FetchOpts{All: root.opts.all, NamePattern: root.opts.name})
			if err != nil {
				return err
			}

			resolvedPath, err = checkFinalPath(resolvedPath, assets.SanitizeName(pResult.Name, pResult.Version))
			if err != nil {
				return err
			}

			hash, err := saveToDisk(pResult, resolvedPath, root.opts.force)
			if err != nil {
				return fmt.Errorf("error installing binary: %w", err)
			}

			// Convert to absolute path before storing in config
			absPath, err := filepath.Abs(resolvedPath)
			if err != nil {
				return fmt.Errorf("error converting to absolute path: %w", err)
			}

			err = config.UpsertBinary(&config.Binary{
				RemoteName:  pResult.Name,
				Path:        absPath,
				Version:     pResult.Version,
				Hash:        fmt.Sprintf("%x", hash),
				URL:         u,
				Provider:    p.GetID(),
				PackagePath: pResult.PackagePath,
				StateURL:    u,
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
	root.cmd.Flags().StringVarP(&root.opts.name, "name", "n", "", "Glob pattern to select a specific asset (use asset/file for archive contents)")
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

// TODO: check if other binary has the same hash and warn about it.
// TODO: if the file is zipped, tared, whatever then extract it
func saveToDisk(f *providers.File, path string, overwrite bool) ([]byte, error) {
	epath := os.ExpandEnv(path)

	dir := filepath.Dir(epath)
	base := filepath.Base(epath)

	// Write to a temp .new file first to allow atomic replacement.
	// This is required on Windows where in-place writes to running binaries fail.
	newPath := filepath.Join(dir, fmt.Sprintf(".%s.new", base))

	file, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o766)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	tr := io.TeeReader(f.Data, h)

	log.Infof("Copying for %s@%s into %s", f.Name, f.Version, epath)
	_, err = io.Copy(file, tr)
	file.Close()
	if err != nil {
		_ = os.Remove(newPath)
		return nil, err
	}

	// If the target already exists, check overwrite flag and move it aside.
	oldPath := filepath.Join(dir, fmt.Sprintf(".%s.old", base))
	_ = os.Remove(oldPath)

	_, statErr := os.Stat(epath)
	if statErr == nil {
		if !overwrite {
			_ = os.Remove(newPath)
			return nil, fmt.Errorf("%w", os.ErrExist)
		}
		log.Debugf("Overwrite flag set, moving %s to %s\n", epath, oldPath)
		if err = os.Rename(epath, oldPath); err != nil {
			_ = os.Remove(newPath)
			return nil, err
		}
	}

	// Atomically move the new file into place.
	if err = os.Rename(newPath, epath); err != nil {
		// Attempt rollback if we moved the old file aside.
		if rerr := os.Rename(oldPath, epath); rerr != nil {
			log.Debugf("Rollback failed, %s may be missing: %v\n", epath, rerr)
		}
		return nil, err
	}

	// Clean up the old file.
	_ = os.Remove(oldPath)

	return h.Sum(nil), nil
}
