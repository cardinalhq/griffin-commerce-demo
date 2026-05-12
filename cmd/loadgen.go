// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/loadgen"
	"github.com/spf13/cobra"
)

var loadgenCmd = &cobra.Command{
	Use:   "loadgen",
	Short: "Run a continuous traffic generator against Griffin (SmartHub flow)",
	Long: `Sustains a low-rate stream of checkout sessions against the cart service so
the customer-persona side of the Airtel demo has continuous traffic to investigate.

Each iteration: create cart → add 1-2 items → checkout. The checkout call
fans out to payment + shipping internally, so every iteration produces a
multi-service trace. When the dbaas.disk-full knob is active in the
controlplane, the payment leg of these traces returns SQLSTATE 53100,
which is what the customer-side investigate flow surfaces.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Loadgen...")
		if err := loadgen.Start(); err != nil {
			log.Fatalf("Failed to start loadgen: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(loadgenCmd)
}
