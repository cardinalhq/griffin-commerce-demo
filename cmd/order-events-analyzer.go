// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	orderevents "github.com/cardinalhq/griffin-commerce-demo/services/order-events-analyzer"
	"github.com/spf13/cobra"
)

var orderEventsAnalyzerCmd = &cobra.Command{
	Use:   "order-events-analyzer",
	Short: "Run the simulated order-events consumer-lag emitter",
	Long: `Simulate an order.created event-stream consumer whose lag spikes on a
fixed 3-hour cycle, with correlated error logs gated on the same window —
no admin/controlplane knob required. Designed so any 6-hour lookback always
contains at least one full lag spike-and-recover for the demo's
logs-to-metrics correlation story.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting order-events-analyzer...")
		if err := orderevents.Start(); err != nil {
			log.Fatalf("Failed to start order-events-analyzer: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(orderEventsAnalyzerCmd)
}
