package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/images"
	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Run the image service",
	Long:  `Start the image service that serves product images and static assets`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Images Service...")
		if err := images.Start(); err != nil {
			log.Fatalf("Failed to start images service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(imagesCmd)
}
