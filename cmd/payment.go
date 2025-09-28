package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/payment"
	"github.com/spf13/cobra"
)

var paymentCmd = &cobra.Command{
	Use:   "payment",
	Short: "Run the payment processing service",
	Long:  `Start the payment processing service that handles payment transactions and processing`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Payment Service...")
		if err := payment.Start(); err != nil {
			log.Fatalf("Failed to start payment service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(paymentCmd)
}
