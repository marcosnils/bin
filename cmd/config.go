package cmd

import (
	"fmt"
	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/spf13/cobra"
)

type cfgCmd struct {
	cmd *cobra.Command
}

type cfgGetCmd struct {
	cmd *cobra.Command
}

type cfgSetCmd struct {
	cmd *cobra.Command
}

func newConfigCmd() *cfgCmd {
	cfg := &cfgCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "config",
		Aliases:       []string{"c"},
		Short:         "Configure bin",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			log.Debug("Run config")
		},
	}

	cmd.AddCommand(
		newConfigGetCmd().cmd,
		newConfigSetCmd().cmd,
	)

	cfg.cmd = cmd
	return cfg
}

func newConfigGetCmd() *cfgGetCmd {
	get := &cfgGetCmd{}
	cmd := &cobra.Command{
		Use:     "get <name>",
		Aliases: []string{"g"},
		Short:   "Get a configuration value",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expexting exactly one argument (got %d)", len(args))
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("Run config get")

			v, err := config.GetValue(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%s", v)
			return nil
		},
	}

	get.cmd = cmd
	return get
}

func newConfigSetCmd() *cfgSetCmd {
	set := &cfgSetCmd{}
	cmd := &cobra.Command{
		Use:     "set <name> <value>",
		Aliases: []string{"s"},
		Short:   "Set a configuration value",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("expecting exactly two arguments (got %d)", len(args))
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("Run config set")

			return config.SetValue(args[0], args[1])
		},
	}

	set.cmd = cmd
	return set
}
