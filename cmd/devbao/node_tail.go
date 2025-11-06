package main

import (
	"fmt"
	"path/filepath"

	"github.com/openbao/devbao/pkg/bao"
	"github.com/openbao/devbao/pkg/utils"

	"github.com/urfave/cli/v2"
)

func BuildNodeTailCommand() *cli.Command {
	c := &cli.Command{
		Name:      "tail",
		ArgsUsage: "<name>",
		Usage:     "tail the logs for the given instance",

		Action: RunNodeTailCommand,
	}

	return c
}

func RunNodeTailCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the instance to tail the logs of")
	}

	name := cCtx.Args().First()
	node := &bao.Node{Name: name}
	dir := node.GetDirectory()

	log := filepath.Join(dir, bao.SERVICE_LOG_NAME)

	return utils.Tail(log, false)
}
