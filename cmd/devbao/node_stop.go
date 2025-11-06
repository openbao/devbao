package main

import (
	"fmt"
	"os"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeStopCommand() *cli.Command {
	c := &cli.Command{
		Name:      "stop",
		Aliases:   []string{"k"},
		ArgsUsage: "<name>",
		Usage:     "stop the named instance if it is running",

		Action: RunNodeStopCommand,
	}

	return c
}

func RunNodeStopCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the name of the instance to stop")
	}

	name := cCtx.Args().First()
	node, err := bao.LoadNode(name)
	if err != nil {
		return err
	}

	if err := node.Exec.ValidateRunning(); err == nil {
		fmt.Printf("stopping node %v / pid %v...\n", name, node.Exec.Pid)
		return node.Kill()
	}

	fmt.Fprintf(os.Stderr, "node %v / pid %v was already stopped\n", name, node.Exec.Pid)

	return nil
}
