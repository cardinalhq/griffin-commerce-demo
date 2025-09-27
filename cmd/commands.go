package cmd

import (
	"github.com/spf13/cobra"
)

// RegisterCommands adds all service commands to the root command
func RegisterCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(catalogCmd)
	rootCmd.AddCommand(cartCmd)
	rootCmd.AddCommand(paymentCmd)
	rootCmd.AddCommand(shippingCmd)
	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(recommendationsCmd)
}