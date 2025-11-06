package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeCleanCommand() *cli.Command {
	c := &cli.Command{
		Name:      "clean",
		Aliases:   []string{"remove", "c"},
		ArgsUsage: "<name>",
		Usage:     "remove the named instance",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Value:   false,
				Usage:   "force removal of node",
			},
		},

		Action: RunNodeCleanCommand,
	}

	return c
}

func RunNodeCleanCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the name of the instance to remove")
	}

	force := cCtx.Bool("force")

	name := cCtx.Args().First()
	node, err := bao.LoadNodeUnvalidated(name)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			fmt.Fprintf(os.Stderr, "node %v was already removed\n", name)
			return nil
		}

		// Some other unknown error.
		if !force {
			return fmt.Errorf("failed to load node to determine state: %w", err)
		} else {
			fmt.Fprintf(os.Stderr, "[warning] failed to load node to determine state: %v\n", err)
			node = &bao.Node{
				Name: name,
			}
		}
	}

	if node.Exec != nil {
		if err := node.Exec.ValidateRunning(); err == nil {
			fmt.Fprintf(os.Stderr, "node %v / pid %v is running, stopping...\n", name, node.Exec.Pid)
			err := node.Kill()
			if err != nil {
				if !force {
					return fmt.Errorf("failed to stop node prior to removal: %w", err)
				}

				fmt.Fprintf(os.Stderr, "[warning] failed to stop node prior to removal: %w\n", err)
			}
		}
	}

	fmt.Printf("cleaning node %v...\n", name)
	return node.Clean(force)
}
