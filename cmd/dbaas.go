// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/dbaas"
	"github.com/spf13/cobra"
)

var dbaasCmd = &cobra.Command{
	Use:   "dbaas",
	Short: "Run the simulated DBaaS metrics emitter",
	Long: `Simulate a multi-tenant managed-Postgres fleet for the Airtel DBaaS demo.

Emits OTLP metrics for a synthetic fleet of DB instances across six demo
customers. Each metric carries customer_id / db_id labels so the same
dashboards fan out across tenants.

Phase 1 emits db.up only; remaining metrics land in Phase 2.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting DBaaS Simulator...")
		if err := dbaas.Start(); err != nil {
			log.Fatalf("Failed to start dbaas simulator: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(dbaasCmd)
}
