package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var (
	appDir            string
	infrastructureDir string
	opsDir            string
	namespace         string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "deployer",
	Short: "DevOps management made easy",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
