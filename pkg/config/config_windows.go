package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/options"
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
	for _, p := range strings.Split(penv, ";") {

		if err := checkDirExistsAndWritable(p); err != nil {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		log.Debugf("%s seems to be a dir and writable, adding option.", p)
		opts = append(opts, options.LiteralStringer(p))

	}

	if len(opts) == 0 {
		return "", errors.New("Automatic path detection didn't return any results")
	}

	choice, err := options.Select("Pick a default download dir: ", opts)
	if err != nil {
		return "", err
	}
	return choice.(fmt.Stringer).String(), nil

}

func checkDirExistsAndWritable(dir string) error {
	log.Debugf("Checking path %s", dir)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return err
	}

	// Check if the user bit is enabled in file permission
	if info.Mode().Perm()&(1<<(uint(7))) == 0 {
		return errors.New(fmt.Sprintf("Dir %s is not writable", dir))
	}
	return nil

}
