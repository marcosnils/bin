package config

import (
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
		log.Debugf("Checking path %s", p)

		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		// Check if the user bit is enabled in file permission
		if info.Mode().Perm()&(1<<(uint(7))) != 0 {
			log.Debugf("%s seems to be a dir and writable, adding option.", p)
			opts = append(opts, options.LiteralStringer(p))
		}

	}

	choice, err := options.Select("Pick a default download dir: ", opts)
	if err != nil {
		return "", err
	}
	return choice.(fmt.Stringer).String(), nil

}
