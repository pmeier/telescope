package cmd

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
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

var validate = validator.New(validator.WithRequiredStructEnabled())

func runFunc(fn func(config.Config) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		var code int
		if err := func() error {
			v := config.NewViper()
			if err := v.ReadAndMergeInConfigs(); err != nil {
				return err
			}

			c := config.New()
			if err := v.Unmarshal(c); err != nil {
				return err
			}
			if err := validate.Struct(c); err != nil {
				return err
			}

			return fn(*c)
		}(); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			code = 1
		}

		os.Exit(code)
	}
}
