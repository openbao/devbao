package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildClusterCleanCommand() *cli.Command {
	c := &cli.Command{
		Name:      "clean",
		Aliases:   []string{"destroy", "c"},
		ArgsUsage: "<name>",
		Usage:     "remove the named instance",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Value:   false,
				Usage:   "force removal of cluster",
			},
		},

		Action: RunClusterCleanCommand,
	}

	return c
}

func RunClusterCleanCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the name of the instance to remove")
	}

	force := cCtx.Bool("force")

	clusterName := cCtx.Args().First()
	cluster, err := bao.LoadClusterUnvalidated(clusterName)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			fmt.Fprintf(os.Stderr, "cluster %v was already removed\n", clusterName)
			return nil
		}

		// Some other unknown error.
		if !force {
			return fmt.Errorf("failed to load cluster to determine state: %w", err)
		} else {
			cluster = &bao.Cluster{
				Name: clusterName,
			}
		}
	}

	fmt.Printf("cleaning cluster %v...\n", clusterName)
	return cluster.Clean(force)
}
