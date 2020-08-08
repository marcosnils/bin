// +build !windows

package config

import (
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
func getDefaultPath() string {
	penv := os.Getenv("PATH")
	log.Debugf("User PATH is [%s]", penv)
	opts := []fmt.Stringer{}
	for _, p := range strings.Split(penv, ":") {
		log.Debugf("Checking path %s", p)

		//TODO make this work in non unix platforms
		err := unix.Access(p, unix.W_OK)

		if err != nil {
			log.Debugf("Error [%s] checking path", err)
			continue
		}

		log.Debugf("%s seems to be a dir and writable, adding option.", p)
		opts = append(opts, options.LiteralStringer(p))

	}

	return options.Select("Pick a default download dir: ", opts).(options.LiteralStringer).String()

}
