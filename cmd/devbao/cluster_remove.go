package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildClusterRemoveCommand() *cli.Command {
	c := &cli.Command{
		Name:      "remove",
		Aliases:   []string{"r"},
		ArgsUsage: "<cluster-name> <node-name>",
		Usage:     "remove a given node from the cluster",

		Action: RunClusterRemoveCommand,
	}

	c.Flags = append(c.Flags, ClusterFlags()...)

	return c
}

func RunClusterRemoveCommand(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 2 {
		return fmt.Errorf("missing required positional argument:\n\t<cluster-name>, the name of the cluster to shrink\n\t<node-name> the name of the node to remove from the cluster")
	}

	clusterName := cCtx.Args().First()
	nodeName := cCtx.Args().Get(1)

	cluster, err := bao.LoadCluster(clusterName)
	if err != nil {
		return fmt.Errorf("error loading cluster: %w", err)
	}

	node, err := bao.LoadNode(nodeName)
	if err != nil {
		return fmt.Errorf("error loading node: %w", err)
	}

	if node.Cluster != clusterName {
		return fmt.Errorf("refusing to remove node not in the cluster (`%v`)", node.Cluster)
	}

	if err := cluster.RemoveNodeHACluster(node); err != nil {
		return fmt.Errorf("failed to remove node from the cluster: %w", err)
	}

	return nil
}
