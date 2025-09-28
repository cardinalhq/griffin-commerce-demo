package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/catalog"
	"github.com/spf13/cobra"
)

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Run the product catalog service",
	Long:  `Start the product catalog service that manages product information and inventory`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Catalog Service...")
		if err := catalog.Start(); err != nil {
			log.Fatalf("Failed to start catalog service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(catalogCmd)
}
