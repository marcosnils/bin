package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/fatih/color"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

func Execute(version string, exit func(int), args []string) {
	// enable colored output on travis
	if os.Getenv("CI") != "" {
		color.NoColor = false
	}

	log.SetHandler(cli.Default)

	// fmt.Println()
	// defer fmt.Println()
	newRootCmd(version, exit).Execute(args)
}

func (cmd *rootCmd) Execute(args []string) {
	cmd.cmd.SetArgs(args)

	if defaultCommand(cmd.cmd, args) {
		cmd.cmd.SetArgs(append([]string{"list"}, args...))
	}

	if err := cmd.cmd.Execute(); err != nil {
		code := 1
		msg := "command failed"
		if eerr, ok := err.(*exitError); ok {
			code = eerr.code
			if eerr.details != "" {
				msg = eerr.details
			}
		}
		log.WithError(err).Error(msg)
		cmd.exit(code)
	}
}

type rootCmd struct {
	cmd   *cobra.Command
	debug bool
	exit  func(int)
}

func newRootCmd(version string, exit func(int)) *rootCmd {
	root := &rootCmd{
		exit: exit,
	}
	cmd := &cobra.Command{
		Use:           "bin",
		Short:         "Effortless binary manager",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if root.debug {
				log.SetLevel(log.DebugLevel)
				log.Debugf("debug logs enabled, version: %s\n", version)
			}

			// check and load config after handlers are configured
			err := config.CheckAndLoad()
			if err != nil {
				log.Fatalf("Error loading config file %v", err)
			}
		},
	}

	cmd.PersistentFlags().BoolVar(&root.debug, "debug", false, "Enable debug mode")
	cmd.AddCommand(
		newInstallCmd().cmd,
		newEnsureCmd().cmd,
		newUpdateCmd().cmd,
		newRemoveCmd().cmd,
		newListCmd().cmd,
		newPruneCmd().cmd,
	)

	root.cmd = cmd
	return root
}

func defaultCommand(cmd *cobra.Command, args []string) bool {
	// find current cmd, if its not root, it means the user actively
	// set a command, so let it go
	xmd, _, _ := cmd.Find(args)
	if xmd != cmd {
		return false
	}

	// if we have != 1 args, assume its a ls
	if len(args) != 1 {
		return true
	}

	// given that its 1, check if its one of the valid standalone flags
	// for the root cmd
	for _, s := range []string{"-h", "--help", "-v", "--version"} {
		if s == args[0] {
			// if it is, we should run the root cmd
			return false
		}
	}

	// otherwise, we should probably prepend ls
	return true
}

func getBinPath(name string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}

	cfg := config.Get()

	for _, bin := range cfg.Bins {
		if bin.RemoteName == name {
			return bin.Path, nil
		}
	}

	return "", fmt.Errorf("binary path %s not found", name)
}
