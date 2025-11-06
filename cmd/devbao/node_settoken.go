package main

import (
	"fmt"
	"os"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeSetTokenCommand() *cli.Command {
	c := &cli.Command{
		Name:      "set-token",
		Aliases:   []string{"s-t"},
		ArgsUsage: "<name> <token>",
		Usage:     "save root token for future use",

		Action: RunNodeSetTokenCommand,
	}

	return c
}

func RunNodeSetTokenCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which has this token\n\t<token>, the root token to be saved")
	}

	name := cCtx.Args().First()
	token := cCtx.Args().Get(1)

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	validated, err := node.SetToken(token)
	if err != nil {
		return err
	}

	if !validated {
		fmt.Fprintf(os.Stderr, "[warning] instance (%v) was not running; could not validate provided token\n", name)
	}

	return node.SaveConfig()
}
