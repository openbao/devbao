package main

import (
	"fmt"

	"github.com/cipherboy/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildClusterUnsealCommand() *cli.Command {
	c := &cli.Command{
		Name:      "unseal",
		Aliases:   []string{"u"},
		ArgsUsage: "<cluster-name>",
		Usage:     "unseals all node in a given cluster",

		Action: RunClusterUnsealCommand,
	}

	return c
}

func RunClusterUnsealCommand(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 1 {
		return fmt.Errorf("missing required positional argument:\n\t<cluster-name>, the name of the cluster to extend")
	}

	clusterName := cCtx.Args().First()

	cluster, err := bao.LoadCluster(clusterName)
	if err != nil {
		return fmt.Errorf("error loading cluster: %w", err)
	}

	for index, nodeName := range cluster.Nodes {
		node, err := bao.LoadNode(nodeName)
		if err != nil {
			return fmt.Errorf("error loading node [%d/%v]: %w", index, nodeName, err)
		}

		unsealed, err := node.Unseal()
		if err != nil {
			return fmt.Errorf("error unsealing node [%d/%v]: %w", index, nodeName, err)
		}

		if !unsealed {
			return fmt.Errorf("failed to fully unseal node [%d/%v]", index, nodeName)
		}
	}

	return nil
}
