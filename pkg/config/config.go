package config

import (
	"encoding/json"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

var cfg config

type config struct {
	Bins []*Binary `json:"bins"`
}

type Binary struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
	URL     string `json:"url"`
}

func CheckAndLoad() error {
	u, _ := user.Current()
	f, err := os.OpenFile(filepath.Join(u.HomeDir, ".bin/config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	defer f.Close()

	err = json.NewDecoder(f).Decode(&cfg)

	// ignore if file is empty
	if err != nil && err != io.EOF {
		return err
	}

	return nil

}

func Get() *config {
	return &cfg
}

func AddBinary(c *Binary) error {

	if c != nil {
		cfg.Bins = append(cfg.Bins, c)
		err := write()
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveBinaries removes the specified paths
// from bin configuration. It doesn't care about the order
func RemoveBinaries(paths []string) error {
	if len(paths) > 0 {
		k := 0
		for _, cb := range cfg.Bins {
			for _, p := range paths {
				if cb.Path != p {
					cfg.Bins[k] = cb
					k++
				}
			}
		}

		cfg.Bins = cfg.Bins[:k]

		return write()
	}

	return nil
}

func write() error {
	u, _ := user.Current()
	f, err := os.OpenFile(filepath.Join(u.HomeDir, ".bin/config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	err = json.NewEncoder(f).Encode(cfg)

	if err != nil {
		return err
	}

	return nil
}

// GetArch is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.
func GetArch() string {
	return runtime.GOARCH
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() string {
	return runtime.GOOS
}
