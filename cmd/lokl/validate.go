package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		fmt.Printf("✓ %s is valid\n", configFile)
		return nil
	},
}
