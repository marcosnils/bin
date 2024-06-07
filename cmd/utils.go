package cmd

import (
	"strings"

	"github.com/marcosnils/bin/pkg/config"
)

func checkBinExistsInConfig(url string, bins map[string]*config.Binary) string {
	if strings.Contains(url, "releases/tag") {
		return ""
	}

	u := strings.Split(url, "/")
	if len(u) == 0 {
		return ""
	}

	binName := u[len(u)-1]

	for _, bin := range bins {
		if bin.RemoteName == binName {
			return binName
		}
	}

	return ""
}
