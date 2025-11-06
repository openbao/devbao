package main

import (
	"fmt"
	"strings"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildClusterListCommand() *cli.Command {
	c := &cli.Command{
		Name:    "list",
		Aliases: []string{"l"},
		Usage:   "list known clusters",

		Action: RunClusterListCommand,
	}

	return c
}

func RunClusterListCommand(cCtx *cli.Context) error {
	if cCtx.Args().Present() {
		return fmt.Errorf("unexpected positional argument -- this command takes none: `%v`", cCtx.Args().First())
	}

	clusters, err := bao.ListClusters()
	if err != nil {
		return err
	}

	var lines []string
	for index, name := range clusters {
		cluster, err := bao.LoadCluster(name)
		if err != nil {
			return fmt.Errorf("failed to load cluster %d (`%v`): %w", index, name, err)
		}

		lines = append(lines, fmt.Sprintf(" - %v (%v)", name, cluster.Type))

		for _, name := range cluster.Nodes {
			lines = append(lines, fmt.Sprintf("   - node: %v", name))
		}
	}

	fmt.Println(strings.Join(lines, "\n"))

	return nil
}
