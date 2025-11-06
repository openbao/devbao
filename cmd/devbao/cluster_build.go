package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func ClusterFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Value:   false,
			Usage:   "overwrite an existing node, if present",
		},
	}
}

func BuildClusterBuildCommand() *cli.Command {
	c := &cli.Command{
		Name:      "build",
		Aliases:   []string{"b"},
		ArgsUsage: "<cluster-name> <node-name>",
		Usage:     "build a new cluster with the given base node",

		Action: RunClusterBuildCommand,
	}

	c.Flags = append(c.Flags, ClusterFlags()...)

	return c
}

func RunClusterBuildCommand(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 2 {
		return fmt.Errorf("missing required positional argument:\n\t<cluster-name>, the name of the cluster to create\n\t<node-name> the name of the node to add")
	}

	clusterName := cCtx.Args().First()
	nodeName := cCtx.Args().Get(1)

	force := cCtx.Bool("force")

	if !force {
		present, err := bao.ClusterExists(clusterName)
		if err != nil {
			return fmt.Errorf("error checking if cluster exists: %w", err)
		}

		if present {
			return fmt.Errorf("refusing to override cluster %v", clusterName)
		}
	}

	cluster, err := bao.BuildHACluster(clusterName, nodeName)
	if err != nil {
		return fmt.Errorf("failed to build cluster: %w", err)
	}

	if cluster == nil {
		return fmt.Errorf("nil cluster: %w", err)
	}

	return nil
}
