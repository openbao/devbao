package main

import (
	"fmt"
	"os"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeUnsealCommand() *cli.Command {
	c := &cli.Command{
		Name:      "unseal",
		Aliases:   []string{"u"},
		ArgsUsage: "<name>",
		Usage:     "unseals the specified node using locally stored unseal keys",

		Action: RunNodeUnsealCommand,
	}

	return c
}

func RunNodeUnsealCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which should be unsealed")
	}

	name := cCtx.Args().First()

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	unsealed, err := node.Unseal()
	if err != nil {
		return err
	}

	if !unsealed {
		fmt.Fprintf(os.Stderr, "[warning] node %v was already unsealed\n", name)
	}

	return nil
}
