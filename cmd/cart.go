package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/cart"
	"github.com/spf13/cobra"
)

var cartCmd = &cobra.Command{
	Use:   "cart",
	Short: "Run the shopping cart service",
	Long:  `Start the shopping cart service that manages user shopping carts`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Cart Service...")
		if err := cart.Start(); err != nil {
			log.Fatalf("Failed to start cart service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(cartCmd)
}
