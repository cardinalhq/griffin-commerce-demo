// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/solar"
	"github.com/spf13/cobra"
)

var solarCmd = &cobra.Command{
	Use:   "solar",
	Short: "Run the Adani Khavda solar farm telemetry simulator",
	Long: `Simulate the Adani Khavda Renewable Energy Park solar farm for the
Adani Renewable Ops demo.

Emits OTLP metrics and logs for the spec §4 entity catalog (6 blocks,
4 MV transformers, 24 inverter stations, 96 inverters, 12 met stations,
6 trackers, plus 3 PPAs and a 220/400 kV pooling substation).

A local HTTP server on :9999 lets a demo operator activate one of four
failure profiles (mv_transformer_winding_overheat being the canonical
shared-infra blast-radius story).`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Adani Solar Simulator...")
		if err := solar.Start(); err != nil {
			log.Fatalf("Failed to start solar simulator: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(solarCmd)
}
