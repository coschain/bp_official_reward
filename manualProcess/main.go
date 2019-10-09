package main

import (
	"bp_official_reward/manualProcess/command"
	"github.com/coschain/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "manual process",
	Short: "manual process is service to modify processed status of failed swap record",
}

func addCommands() {
	rootCmd.AddCommand(command.StartCmd())
}

func main()  {
	addCommands()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}