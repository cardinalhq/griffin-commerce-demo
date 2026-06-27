// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cmd

import (
	"log"

	"github.com/cardinalhq/griffin-commerce-demo/services/nvcf"
	"github.com/spf13/cobra"
)

var nvcfCmd = &cobra.Command{
	Use:   "nvcf",
	Short: "Run the simulated NVIDIA Cloud Functions metrics emitter",
	Long: `Simulate an NVIDIA Cloud Functions deployment for the NVCF Cardinal demo.

Emits OTLP metrics with the verbatim NVCF native metric names and label
vocabulary (function_id, function_version_id, nvca_cluster_name,
account_name, instance_type, ...) for a synthesized fleet of 4 functions
× 2 versions × 4 accounts × 2 clusters × 24 instances. A local HTTP
server on :9998 exposes /faults/{activate,clear,status,profiles} so a
demo operator can flip between the 11 chaos knobs.

Per docs/specs/nvcf.md.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting NVCF Simulator...")
		if err := nvcf.Start(); err != nil {
			log.Fatalf("Failed to start nvcf simulator: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(nvcfCmd)
}
