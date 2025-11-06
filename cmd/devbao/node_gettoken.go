package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeGetTokenCommand() *cli.Command {
	c := &cli.Command{
		Name:      "get-token",
		Aliases:   []string{"g-t"},
		ArgsUsage: "<name>",
		Usage:     "gets the root token",

		Action: RunNodeGetTokenCommand,
	}

	return c
}

func RunNodeGetTokenCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which has this token")
	}

	name := cCtx.Args().First()

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	fmt.Println(node.Token)
	return nil
}
