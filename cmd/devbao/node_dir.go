package main

import (
	"fmt"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeDirCommand() *cli.Command {
	c := &cli.Command{
		Name:      "dir",
		ArgsUsage: "<name>",
		Usage:     "print the expected directory for the given instance",

		Action: RunNodeDirCommand,
	}

	return c
}

func RunNodeDirCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the instance to print the directory of")
	}

	name := cCtx.Args().First()
	node := &bao.Node{Name: name}
	dir := node.GetDirectory()

	fmt.Println(dir)
	return nil
}
