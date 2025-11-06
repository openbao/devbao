package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeSealCommand() *cli.Command {
	c := &cli.Command{
		Name:      "seal",
		Aliases:   []string{"x"},
		ArgsUsage: "<name>",
		Usage:     "seals the specified node",

		Action: RunNodeSealCommand,
	}

	return c
}

func RunNodeSealCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which should be sealed")
	}

	name := cCtx.Args().First()

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	client, err := node.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node: %w", err)
	}

	if err := client.Sys().Seal(); err != nil {
		return fmt.Errorf("failed to seal specified node %v: %w", name, err)
	}

	return nil
}
