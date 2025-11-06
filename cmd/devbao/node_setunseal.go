package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeSetUnsealCommand() *cli.Command {
	c := &cli.Command{
		Name:      "set-unseal",
		Aliases:   []string{"s-u"},
		ArgsUsage: "<name> <key> [<key> ...]",
		Usage:     "save unseal keys for future use",

		Action: RunNodeSetUnsealCommand,
	}

	return c
}

func RunNodeSetUnsealCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument:\n\t<name>, the node which has these unseal keys\n\t<key>, the unseal key to be saved; can be specified multiple times")
	}

	name := cCtx.Args().First()

	var keys []string
	for i := 1; i < cCtx.Args().Len(); i++ {
		keys = append(keys, cCtx.Args().Get(i))
	}

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	// Unlike root tokens, unseal keys can't be validated without sealing
	// and subsequently unsealing OpenBao. The issue is that this is
	// destructive if the original unseal keys are not retained.
	node.UnsealKeys = keys

	return node.SaveConfig()
}
