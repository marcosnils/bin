//go:build !windows
// +build !windows

package config

import (
	"bufio"
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
	opts := []fmt.Stringer{}
	for _, p := range strings.Split(penv, ":") {
		log.Debugf("Checking path %s", p)

		err := checkDirExistsAndWritable(p)

		if err != nil {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		log.Debugf("%s seems to be a dir and writable, adding option.", p)
		opts = append(opts, options.LiteralStringer(p))

	}

	if len(opts) == 0 {

		for {
			log.Info("Could not find a PATH directory automatically, falling back to manualy selection")
			reader := bufio.NewReader(os.Stdin)
			var response string
			fmt.Printf("\nPlease specify a download directory: ")
			response, err := reader.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("Invalid input")
			}

			if err = checkDirExistsAndWritable(strings.TrimSpace(response)); err != nil {
				log.Debugf("Could not set download directory [%s]: [%v]", response, err)
				// Keep looping until writable and existing dir is selected
				continue
			}

			return response, nil
		}

	}

	choice, err := options.Select("Pick a default download dir: ", opts)
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
