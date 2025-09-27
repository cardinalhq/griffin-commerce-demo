package main

import (
	"fmt"
	"os"

	"github.com/cardinalhq/griffin-commerce-demo/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "griffin",
	Short: "Griffin Commerce Demo - Microservices E-commerce Platform",
	Long: `Griffin Commerce Demo is a microservices-based e-commerce platform
that demonstrates modern cloud-native architecture patterns.

Use the subcommands to run individual services:
  griffin catalog - Run the product catalog service
  griffin cart - Run the shopping cart service
  griffin payment - Run the payment service
  griffin shipping - Run the shipping service
  griffin images - Run the image service
  griffin recommendations - Run the recommendations service`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Register all service commands
	cmd.RegisterCommands(rootCmd)
}