package main

import (
	"fmt"
	"path/filepath"

	"github.com/openbao/devbao/pkg/bao"
	"github.com/openbao/devbao/pkg/utils"

	"github.com/urfave/cli/v2"
)

func BuildNodeTailAuditCommand() *cli.Command {
	c := &cli.Command{
		Name:      "tail-audit",
		ArgsUsage: "<name>",
		Usage:     "tail the audit logs for the given instance",

		Action: RunNodeTailAuditCommand,
	}

	return c
}

func RunNodeTailAuditCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the instance to tail the audit logs of")
	}

	name := cCtx.Args().First()
	node := &bao.Node{Name: name}
	dir := node.GetDirectory()

	log := filepath.Join(dir, "audit.log")
	return utils.Tail(log, false)
}
