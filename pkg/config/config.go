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
	Bins map[string]*Binary `json:"bins"`
}

type Binary struct {
	Path       string `json:"path"`
	RemoteName string `json:"remote_name"`
	Version    string `json:"version"`
	Hash       string `json:"hash"`
	URL        string `json:"url"`
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
	} else if err == io.EOF {
		cfg.Bins = map[string]*Binary{}
	}

	return nil

}

func Get() *config {
	return &cfg
}

//UpsertBinary adds or updats an existing
//binary resource in the config
func UpsertBinary(c *Binary) error {

	if c != nil {
		cfg.Bins[c.Path] = c
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
	for _, p := range paths {
		delete(cfg.Bins, p)
	}

	return write()
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
