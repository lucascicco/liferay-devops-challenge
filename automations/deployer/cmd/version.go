package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Deployer version",
	Long:  `Show Deployer version.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Deployer version: 1.0.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
