// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/shipping"
	"github.com/spf13/cobra"
)

var shippingCmd = &cobra.Command{
	Use:   "shipping",
	Short: "Run the shipping service",
	Long:  `Start the shipping service that handles shipping rates and shipment creation`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Shipping Service...")
		if err := shipping.Start(); err != nil {
			log.Fatalf("Failed to start shipping service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shippingCmd)
}
