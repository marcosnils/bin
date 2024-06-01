package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apex/log"
)

var cfg config

type config struct {
	// DefaultPath might not be expanded so it's important that
	// the caller expands this variable with os.ExpandEnv(string)
	// if necessary
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
	// if file is installed from a package format (zip, tar, etc) store
	// the package path in config so we don't ask the user to select
	// the path again when upgrading
	PackagePath string `json:"package_path"`
}

func ForceInstallationDir() string {
	exeDir := os.Getenv("BIN_EXE_DIR")
	if len(exeDir) == 0 {
		return ""
	}

	if os.MkdirAll(exeDir, 0755); checkDirExistsAndWritable(exeDir) != nil {
		return ""
	}

	return exeDir
}

func CheckAndLoad() error {
	configDir, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.Mkdir(configDir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating config directory [%v]", err)
	}

	log.Debugf("Config directory is: %s", configDir)
	f, err := os.OpenFile(filepath.Join(configDir, "config.json"), os.O_RDWR|os.O_CREATE, 0664)
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
		if exeDir := ForceInstallationDir(); len(exeDir) > 0 {
			cfg.DefaultPath = exeDir
		} else {
			cfg.DefaultPath, err = getDefaultPath()
			if err != nil {
				for {
					log.Info("Could not find a PATH directory automatically, falling back to manual selection")
					reader := bufio.NewReader(os.Stdin)
					var response string
					fmt.Printf("\nPlease specify a download directory: ")
					response, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("invalid input")
					}
					response = strings.TrimSpace(response)

					if err = checkDirExistsAndWritable(response); err != nil {
						log.Debugf("Could not set download directory [%s]: [%v]", response, err)
						// Keep looping until writable and existing dir is selected
						continue
					}

					cfg.DefaultPath = response
					break
				}
			}
		}
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

// UpsertBinary adds or updats an existing
// binary resource in the config
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
	configDir, err := getConfigPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(configDir, "config.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}

	defer f.Close()

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
		// Adding x86_64 manually since the uname syscall (man 2 uname)
		// is not implemented in all systems
		res = append(res, "x86_64")
		res = append(res, "x64")
		res = append(res, "64")
	}
	return res
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() []string {
	res := []string{runtime.GOOS}
	if runtime.GOOS == "windows" {
		// Adding win since some repositories release with that as the indicator of a windows binary
		res = append(res, "win")
	}
	return res
}

// getConfigPath returns the path to the configuration directory respecting
// the `XDG Base Directory specification` using the following strategy:
//   - to prevent breaking of existing configurations, check if "$HOME/.bin/config.json"
//     exists and return "$HOME/.bin"
//   - if "XDG_CONFIG_HOME" is set, return "$XDG_CONFIG_HOME/bin"
//   - if "$HOME/.config" exists, return "$home/.config/bin"
//   - default to "$HOME/.bin/"
//
// ToDo: move the function to config_unix.go and add a similar function for windows,
//
//	%APPDATA% might be the right place on windows
func getConfigPath() (string, error) {
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		if _, err := os.Stat(filepath.Join(home, ".bin", "config.json")); !os.IsNotExist(err) {
			return filepath.Join(path.Join(home, ".bin")), nil
		}
	}

	c := os.Getenv("XDG_CONFIG_HOME")
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		return filepath.Join(c, "bin"), nil
	}

	if homeErr != nil {
		return "", homeErr
	}
	c = filepath.Join(home, ".config")
	if _, err := os.Stat(c); !os.IsNotExist(err) {
		return filepath.Join(c, "bin"), nil
	}

	return filepath.Join(home, ".bin"), nil
}

func GetOSSpecificExtensions() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{"AppImage"}
	case "windows":
		return []string{"exe"}
	default:
		return nil
	}
}
