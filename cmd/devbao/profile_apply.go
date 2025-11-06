package main

import (
	"fmt"
	"os"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildProfileApplyCommand() *cli.Command {
	c := &cli.Command{
		Name:      "apply",
		Aliases:   []string{"a"},
		ArgsUsage: "<name> <profile>",
		Usage:     "apply configuration to a given instance",

		Action: RunProfileApplyCommand,
	}

	return c
}

func RunProfileApplyCommand(cCtx *cli.Context) error {
	if len(cCtx.Args().Slice()) != 2 {
		return fmt.Errorf("missing required positional argument: instance name and profile\nUsage: devbao profile apply <name> <position>")
	}

	name := cCtx.Args().First()
	profile := cCtx.Args().Get(1)

	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	client, err := node.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node %v: %w", name, err)
	}

	warnings, err := bao.ProfileSetup(client, profile)
	for index, warning := range warnings {
		fmt.Fprintf(os.Stderr, " - [warning %d]: %v\n", index, warning)
	}
	return err
}
