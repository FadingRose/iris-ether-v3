package main

import (
	"fadingrose/iris-ether-v3/cmd/main/cmds"

	"github.com/spf13/cobra"
)

var (
	provider string // bytecode source from database
	output   string
	timeout  int
)

var rootCmd = &cobra.Command{
	Use:   "iris-ether <bytecode-file> | <contract-address> --provider <.db>",
	Short: "CFG Analyzer analyzes bytecode and generates control flow graphs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmds.Run(cmd, args[0], provider, timeout)
	},
}

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "v3 add liveness mechanism",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmds.RunBaseline()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&provider, "provider", "p", "", "input from database path")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "output database path")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "timeout in seconds")

	rootCmd.AddCommand(baselineCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		return
	}
}
