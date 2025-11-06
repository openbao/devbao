package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeInitializeCommand() *cli.Command {
	c := &cli.Command{
		Name:      "initialize",
		Aliases:   []string{"i"},
		ArgsUsage: "<name>",
		Usage:     "initializes the specified node; equivalent to operator init with default arguments (3 shares, 2 required)",

		Action: RunNodeInitializeCommand,
	}

	return c
}

func RunNodeInitializeCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which should be initialized")
	}

	name := cCtx.Args().First()

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	if err := node.Exec.ValidateRunning(); err != nil {
		return fmt.Errorf("specified node is not running: %w", err)
	}

	return node.Initialize()
}
