package main

import (
	"fmt"
	"strings"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeListCommand() *cli.Command {
	c := &cli.Command{
		Name:    "list",
		Aliases: []string{"l"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "state",
				Value: "",
				Usage: "only return instances in the given state; `` for all, `running` for running instances, and `stopped` for stopped instances",
			},
		},
		Usage: "list running and stopped nodes",

		Action: RunNodeListCommand,
	}

	return c
}

func RunNodeListCommand(cCtx *cli.Context) error {
	if cCtx.Args().Present() {
		return fmt.Errorf("unexpected positional argument -- this command takes none: `%v`", cCtx.Args().First())
	}

	nodes, err := bao.ListNodes()
	if err != nil {
		return err
	}

	filterState := cCtx.String("state")
	if filterState != "" && filterState != "running" && filterState != "stopped" {
		return fmt.Errorf("unknown value for -state: valid values are ``, `running`, and `stopped`; got `%v`", filterState)
	}

	var lines []string
	for index, name := range nodes {
		node, err := bao.LoadNode(name)
		if err != nil {
			return fmt.Errorf("failed to load node %d (`%v`): %w", index, name, err)
		}

		state := "stopped"
		if node.Exec != nil {
			if err := node.Exec.ValidateRunning(); err == nil {
				state = "running"
			}
		}

		if filterState != "" && state != filterState {
			continue
		}

		cluster := ""
		if node.Cluster != "" {
			cluster = fmt.Sprintf(" [cluster: %v]", node.Cluster)
		}

		lines = append(lines, fmt.Sprintf(" - %v (%v)%v", name, state, cluster))
	}

	fmt.Println(strings.Join(lines, "\n"))

	return nil
}
