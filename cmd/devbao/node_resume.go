package main

import (
	"fmt"
	"os"
	"time"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeResumeCommand() *cli.Command {
	c := &cli.Command{
		Name:      "resume",
		Aliases:   []string{"r"},
		ArgsUsage: "<name>",
		Usage:     "resume the named instance if it is not running",

		Action: RunNodeResumeCommand,
	}

	c.Flags = append(c.Flags, UnsealFlags()...)

	return c
}

func RunNodeResumeCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the name of the instance to resume")
	}

	name := cCtx.Args().First()
	node, err := bao.LoadNode(name)
	if err != nil {
		return err
	}

	unseal := cCtx.Bool("unseal")

	if err := node.Exec.ValidateRunning(); err == nil {
		fmt.Fprintf(os.Stderr, "node %v / pid %v is already running\n", name, node.Exec.Pid)
		return nil
	}

	if node.Config.Dev != nil {
		fmt.Fprintf(os.Stderr, "warning: node %v is a dev mode instance; this means its storage was not persistent and will have different state\n", name)
	}

	fmt.Printf("resuming node %v...\n", name)
	if err := node.Resume(); err != nil {
		return err
	}

	if unseal {
		if node.Config.Dev != nil {
			fmt.Fprintf(os.Stderr, "warning: node %v is a dev mode instance; it was automatically unsealed with fresh seal keys\n", name)
			return nil
		}

		if len(node.UnsealKeys) == 0 {
			return fmt.Errorf("instance was started but had no stored unseal keys so unable to automatically unseal")
		}

		if _, err := node.Unseal(); err != nil {
			return fmt.Errorf("failed to unseal node: %w", err)
		}

		// TODO: use a client request with proper back-off to determine
		// when the node is responding.
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}
