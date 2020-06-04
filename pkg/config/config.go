package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apex/log"
	"golang.org/x/sys/unix"
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
}

func CheckAndLoad() error {
	u, _ := user.Current()
	f, err := os.OpenFile(filepath.Join(u.HomeDir, ".bin/config.json"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	defer f.Close()

	err = json.NewDecoder(f).Decode(&cfg)

	if len(cfg.DefaultPath) == 0 {
		cfg.DefaultPath = getDefaultPath()
	}
	log.Debugf("Download path set to %s", cfg.DefaultPath)
	// ignore if file is empty
	if err != nil && err != io.EOF {
		return err
	} else if err == io.EOF {
		cfg.Bins = map[string]*Binary{}
	}

	return nil

}

//getDefaultPath reads the user's PATH variable
//and returns the first directory that's writable by the current
//user in the system
//TODO add feature to prompt the user which to select
//if many paths are found
func getDefaultPath() string {
	penv := os.Getenv("PATH")
	log.Debugf("User PATH is [%s]", penv)
	options := []string{}
	for _, p := range strings.Split(penv, ":") {
		log.Debugf("Checking path %s", p)

		//TODO make this work in non unix platforms
		err := unix.Access(p, unix.W_OK)

		if err != nil {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		log.Debugf("%s seems to be a dir and writable, adding option.", p)
		options = append(options, p)

	}

	return selectOption("Pick a default download dir: ", options)

}

//selectOptions prompts the user which
//of the available options is the desired
//through STDIN
func selectOption(msg string, opts []string) string {
	if len(opts) == 1 {
		return opts[0]
	}
	fmt.Print(msg)
	for i, o := range opts {
		fmt.Printf("\n [%d] %s", i+1, o)
	}

	var opt uint
	var err error
	for {
		fmt.Printf("\n Select an option: ")
		_, err = fmt.Scanln(&opt)
		if err != nil || opt < 1 || int(opt) > len(opts) {
			fmt.Printf("Invalid option")
			continue
		}
		break

	}

	return opts[opt-1]
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
func GetArch() []string {
	res := []string{runtime.GOARCH}
	if runtime.GOARCH == "amd64" {
		//Adding x86_64 manually since the uname syscall (man 2 uname)
		//is not implemented in all systems
		res = append(res, "x86_64")
	}
	return res
}

// GetOS is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
func GetOS() []string {
	return []string{runtime.GOOS}
}
