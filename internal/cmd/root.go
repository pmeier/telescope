package cmd

import (
	"fmt"
	"os"

	"github.com/pmeier/telescope/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "telescope",
	Short: "Observe Sungrow plant",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runFunc(fn func(config.Config) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		var code int
		if err := func() error {
			c, err := config.Load()
			if err != nil {
				return err
			}

			fmt.Printf("%+v\n", c)
			return nil

			// return fn(*c)
		}(); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			code = 1
		}

		os.Exit(code)
	}
}
