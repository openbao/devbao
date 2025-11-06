package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func BuildNodeEnvCommand() *cli.Command {
	c := &cli.Command{
		Name:      "env",
		Aliases:   []string{"e"},
		ArgsUsage: "<name>",
		Usage:     "print environment variables required to connect",

		Action: RunNodeEnvCommand,
	}

	return c
}

func PrintEnv(node string, env map[string]string) error {
	fmt.Printf("# ===== node %v ===== #\n\n", node)

	var envs []string
	for envName := range env {
		envs = append(envs, envName)
	}

	sort.Strings(envs)

	for _, envName := range envs {
		envValue := env[envName]
		fmt.Printf(`export %v="%v"`+"\n", envName, envValue)
	}

	return nil
}

func RunNodeEnvCommand(cCtx *cli.Context) error {
	if !cCtx.Args().Present() {
		return fmt.Errorf("missing required positional argument: <name>, the node whose environment should be printed")
	}

	name := cCtx.Args().First()
	node, err := bao.LoadNode(name)
	if err != nil {
		return fmt.Errorf("failed to load node: %w", err)
	}

	// While not fatal, we want to inform callers that they're sourcing
	// environment for a stopped node.
	if err := node.Exec.ValidateRunning(); err != nil {
		fmt.Fprintf(os.Stderr, "[warning] node %v / pid %v is not running...\n", name, node.Exec.Pid)
	}

	env, err := node.GetEnv()
	if err != nil {
		return err
	}

	return PrintEnv(name, env)
}
