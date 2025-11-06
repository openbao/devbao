package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeGetUnsealCommand() *cli.Command {
	c := &cli.Command{
		Name:      "get-unseal",
		Aliases:   []string{"g-u"},
		ArgsUsage: "<name>",
		Usage:     "gets the unseal keys for the specified node",

		Action: RunNodeGetUnsealCommand,
	}

	return c
}

func RunNodeGetUnsealCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node whose unseal keys should be fetched")
	}

	name := cCtx.Args().First()

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	for _, key := range node.UnsealKeys {
		fmt.Println(key)
	}

	return nil
}
