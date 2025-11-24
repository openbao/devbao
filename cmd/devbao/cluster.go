package main

import (
	"github.com/urfave/cli/v2"
)

func BuildClusterCommand() *cli.Command {
	c := &cli.Command{
		Name:    "cluster",
		Aliases: []string{"clusters", "c"},
		Usage:   "commands for managing clusters",
	}

	c.Subcommands = append(c.Subcommands, BuildClusterBuildCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterCleanCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterJoinCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterListCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterRemoveCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterResumeCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterStartCommand())
	c.Subcommands = append(c.Subcommands, BuildClusterUnsealCommand())

	return c
}
