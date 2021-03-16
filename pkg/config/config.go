package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/apex/log"
)

var cfg config

type config struct {
	DefaultPath string             `json:"default_path"`
	Bins        map[string]*Binary `json:"bins"`
}

type Binary struct {
	Path       string `json:"path"`
	RemoteName string `json:"remote_name"`
	Version    string `json:"version"`
	Hash       string `json:"hash"`
	URL        string `json:"url"`
	Provider   string `json:"provider"`
}

func CheckAndLoad() error {
	u, _ := user.Current()

	configDir := filepath.Join(u.HomeDir, ".bin")

	if err := os.Mkdir(configDir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Error creating config directory [%v]", err)
	}

	f, err := os.OpenFile(filepath.Join(configDir, "config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		if err == io.EOF {
			// Empty file and/or was just created, initialize cfg.Bins
			cfg.Bins = map[string]*Binary{}
		} else {
			return err
		}
	}

	if len(cfg.DefaultPath) == 0 {
		cfg.DefaultPath, err = getDefaultPath()
		if err != nil {
			return err
		}
		f.Close()
		if err := write(); err != nil {
			return err
		}
	}
	log.Debugf("Download path set to %s", cfg.DefaultPath)

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
	f, err := os.OpenFile(filepath.Join(u.HomeDir, ".bin", "config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	decoder := json.NewEncoder(f)
	decoder.SetIndent("", "    ")
	err = decoder.Encode(cfg)

	if err != nil {
		return err
	}

	return nil
}

// GetArch is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.
func GetArch() []string {
	res := []string{runtime.GOARCH}
	if runtime.GOARCH == "amd64" {
		//Adding x86_64 manually since the uname syscall (man 2 uname)
		//is not implemented in all systems
		res = append(res, "x86_64")
		res = append(res, "amd64")
		res = append(res, "64")
	}
	return res
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() []string {
	return []string{runtime.GOOS}
}
