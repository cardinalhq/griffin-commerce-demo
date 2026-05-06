// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/controlplane"
	"github.com/spf13/cobra"
)

var controlplaneCmd = &cobra.Command{
	Use:   "controlplane",
	Short: "Run the fault-injection control plane",
	Long:  `Start the control plane that holds the single active fault-injection knob and serves /admin/faults to per-service polling clients and the admin UI.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting Control Plane Service...")
		if err := controlplane.Start(); err != nil {
			log.Fatalf("Failed to start control plane service: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(controlplaneCmd)
}
