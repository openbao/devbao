package main

import (
	"fmt"
	"os"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeSetAddressCommand() *cli.Command {
	c := &cli.Command{
		Name:      "set-address",
		Aliases:   []string{"s-a"},
		ArgsUsage: "<name> <token>",
		Usage:     "save connection address for future use",

		Action: RunNodeSetAddressCommand,
	}

	return c
}

func RunNodeSetAddressCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which has this token\n\t<address>, the address to save")
	}

	name := cCtx.Args().First()
	addr := cCtx.Args().Get(1)

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	validated, err := node.SetAddress(addr)
	if err != nil {
		return err
	}

	if !validated {
		fmt.Fprintf(os.Stderr, "[warning] instance (%v) was not running; could not validate provided addr\n", name)
	}

	return node.SaveConfig()
}
