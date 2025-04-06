package cmd

import (
	"github.com/pmeier/telescope/internal/observe"
	"github.com/spf13/cobra"
)

var observeCmd = &cobra.Command{
	Use:   "observe",
	Short: "Start observation ",
	Run:   runFunc(observe.Run),
}

func init() {
	rootCmd.AddCommand(observeCmd)
}
