package main

import (
	"fmt"

	"github.com/cipherboy/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildClusterResumeCommand() *cli.Command {
	c := &cli.Command{
		Name:      "resume",
		Aliases:   []string{"re"},
		ArgsUsage: "<cluster-name>",
		Usage:     "resumes all node in a given cluster",

		Action: RunClusterResumeCommand,
	}

	return c
}

func RunClusterResumeCommand(cCtx *cli.Context) error {
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

		err = node.Resume()
		if err != nil {
			return fmt.Errorf("error resuming node [%d/%v]: %w", index, nodeName, err)
		}
	}

	return nil
}
