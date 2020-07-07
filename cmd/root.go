package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gopcp",
	Short: "Go P2P Control Plane",
	Long: `
	Toolkit to build P2P network of nodes which can run golang
	applications. The applications need to be structured in a particular way to
	be able to deployed on this network.
	`,
}

func init() {
	auth.InitializeAuthCommands(rootCmd)
}

// Execute is the root function.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
