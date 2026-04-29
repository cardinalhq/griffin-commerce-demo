// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/recommendations"
	"github.com/spf13/cobra"
)

var recommendationsCmd = &cobra.Command{
	Use:   "recommendations",
	Short: "Run the recommendations service",
	Long:  `Start the recommendations service that provides product recommendations and suggestions`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Recommendations Service...")
		if err := recommendations.Start(); err != nil {
			log.Fatalf("Failed to start recommendations service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(recommendationsCmd)
}
