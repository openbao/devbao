package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"

	"github.com/openbao/devbao/cmd/devbao/tui"
)

func BuildTUICommand() *cli.Command {
	c := &cli.Command{
		Name:    "tui",
		Aliases: []string{"t"},
		Usage:   "interactive user interface for using devbao",

		Action: RunTUICommand,
	}

	return c
}

func RunTUICommand(cCtx *cli.Context) error {
	if _, present := os.LookupEnv("DEBUG"); present {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			return err
		}
		defer f.Close()
	}

	prog := tea.NewProgram(tui.TabModel(), tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return err
	}

	return nil
}
