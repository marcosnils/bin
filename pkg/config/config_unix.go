//go:build !windows
// +build !windows

package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/options"
	"golang.org/x/sys/unix"
)

//getDefaultPath reads the user's PATH variable
//and returns the first directory that's writable by the current
//user in the system
//TODO add feature to prompt the user which to select
//if many paths are found
func getDefaultPath() (string, error) {
	penv := os.Getenv("PATH")
	log.Debugf("User PATH is [%s]", penv)
	opts := map[fmt.Stringer]struct{}{}
	for _, p := range strings.Split(penv, ":") {
		log.Debugf("Checking path %s", p)

		err := checkDirExistsAndWritable(p)

		if err != nil {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		log.Debugf("%s seems to be a dir and writable, adding option.", p)
		opts[options.LiteralStringer(p)] = struct{}{}

	}

	// TODO this logic is also duplicated in the windows config. We should
	// move it to config.go
	if len(opts) == 0 {
		return "", errors.New("Automatic path detection didn't return any results")
	}

	sopts := []fmt.Stringer{}
	for k, _ := range opts {
		sopts = append(sopts, k)
	}

	choice, err := options.Select("Pick a default download dir: ", sopts)
	if err != nil {
		return "", err
	}
	return choice.(fmt.Stringer).String(), nil
}

func checkDirExistsAndWritable(dir string) error {
	if fi, err := os.Stat(dir); err != nil {
		return fmt.Errorf("Error setting download path [%w]", err)
	} else if !fi.IsDir() {
		return errors.New("Download path is not a directory")
	}
	//TODO make this work in non unix platforms
	err := unix.Access(dir, unix.W_OK)
	return err
}
