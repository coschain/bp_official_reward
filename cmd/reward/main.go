package main

import (
	"bp_official_reward/cmd/reward/commands"
	"github.com/coschain/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "BpReward",
	Short: "BpReward is service to distribute reward to voters which voted for cos official block producer",
}

func addCommands() {
	rootCmd.AddCommand(commands.StartCmd())
}

func main()  {
	addCommands()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}