// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

// state.go defines the fleet entity types the synth process tracks.
// All identifiers use the verbatim NVCF label vocabulary from
// docs/specs/nvcf.md §"Resource attribute / label vocabulary".

// Function is one of the seeded NVCF functions (e.g., "chat-helpful").
type Function struct {
	FunctionID    string // UUID string, becomes function_id label
	FunctionName  string // demo-only sugar
	WorkloadType  string // demo-only: streaming | http | grpc
	BaseTTFTSec   float64
	BaseOutputTPS float64 // 0 when not LLM-shaped (e.g., fraud-detect, embed-text)
}

// FunctionVersion is one deployment version of a function (v1 / v2 / ...).
type FunctionVersion struct {
	FunctionVersionID string // UUID string, becomes function_version_id label
	FunctionID        string
	VersionLabel      string // human "v1", "v2" — demo-only sugar
}

// Account is an NVCF tenant.
type Account struct {
	AccountName        string  // account_name label
	AccountDisplayName string  // account_display_name label
	TrafficWeight      float64 // relative weight for synth's invocation distribution
}

// Cluster is a fake GPU cluster.
type Cluster struct {
	NVCAClusterName  string // nvca_cluster_name label
	NVCAClusterGroup string // nvca_cluster_group label
	InstanceType     string // e.g., NCP.GPU.H100_80GB_1x
	NodeCount        int
	GPUsPerNode      int
}

// Instance is one node:gpu pair inside a cluster, addressable for DCGM metrics.
type Instance struct {
	NVCAClusterName string
	NodeIndex       int
	GPUIndex        int
	Device          string // "0", "1", ... for the DCGM `device` label
	Hostname        string // demo-only convenience
	PCIBusID        string // DCGM `pci_bus_id` label
	ModelName       string // DCGM `modelName` label (derived from InstanceType)
}

// InferenceServer is one (function_version × instance) — what NVCF's
// llm-request-router would call an inference_server_id.
type InferenceServer struct {
	InferenceServerID string // becomes inference_server_id label
	FunctionVersionID string
	FunctionID        string
	NVCAClusterName   string
	Device            string
	Model             string // becomes `model` label on stargate_* metrics — matches function_name for the demo
}
