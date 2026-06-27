// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

import "fmt"

// Catalog is the static seeded fleet the synth process emits metrics for.
// Constructed once at startup; never mutated.
type Catalog struct {
	Functions        []*Function
	Versions         []*FunctionVersion
	Accounts         []*Account
	Clusters         []*Cluster
	Instances        []*Instance
	InferenceServers []*InferenceServer

	// Indexes for fast lookup during synth.
	VersionsByFunction map[string][]*FunctionVersion
	ServersByVersion   map[string][]*InferenceServer
	InstancesByCluster map[string][]*Instance
}

// NewCatalog returns the canonical demo fleet:
//
//   - 2 clusters (us-west-2-a A100x16, us-east-1-a H100x8)
//   - 4 functions × 2 versions (with the latency-only/embed functions at v1 only)
//   - 4 accounts
//   - one inference_server_id per (function_version × instance) — but only
//     for functions pinned to that cluster
func NewCatalog() *Catalog {
	c := &Catalog{
		VersionsByFunction: map[string][]*FunctionVersion{},
		ServersByVersion:   map[string][]*InferenceServer{},
		InstancesByCluster: map[string][]*Instance{},
	}

	// Functions. UUIDs deterministic so dashboard variables can default to
	// "the first function" without label drift between runs.
	chatHelpful := &Function{
		FunctionID:    "11111111-1111-1111-1111-111111111111",
		FunctionName:  "chat-helpful",
		WorkloadType:  "streaming",
		BaseTTFTSec:   0.18,
		BaseOutputTPS: 95,
	}
	summarizeDoc := &Function{
		FunctionID:    "22222222-2222-2222-2222-222222222222",
		FunctionName:  "summarize-doc",
		WorkloadType:  "http",
		BaseTTFTSec:   0.41,
		BaseOutputTPS: 70,
	}
	fraudDetect := &Function{
		FunctionID:    "33333333-3333-3333-3333-333333333333",
		FunctionName:  "fraud-detect",
		WorkloadType:  "grpc",
		BaseTTFTSec:   0.022,
		BaseOutputTPS: 0, // latency-only, no token shape
	}
	embedText := &Function{
		FunctionID:    "44444444-4444-4444-4444-444444444444",
		FunctionName:  "embed-text",
		WorkloadType:  "http",
		BaseTTFTSec:   0.035,
		BaseOutputTPS: 0,
	}
	c.Functions = []*Function{chatHelpful, summarizeDoc, fraudDetect, embedText}

	// Versions. v1+v2 for the LLM-shaped ones; v1 only for the simpler ones.
	c.Versions = []*FunctionVersion{
		{FunctionVersionID: "a1111111-1111-1111-1111-111111111111", FunctionID: chatHelpful.FunctionID, VersionLabel: "v1"},
		{FunctionVersionID: "a2222222-2222-2222-2222-222222222222", FunctionID: chatHelpful.FunctionID, VersionLabel: "v2"},
		{FunctionVersionID: "b1111111-1111-1111-1111-111111111111", FunctionID: summarizeDoc.FunctionID, VersionLabel: "v1"},
		{FunctionVersionID: "b2222222-2222-2222-2222-222222222222", FunctionID: summarizeDoc.FunctionID, VersionLabel: "v2"},
		{FunctionVersionID: "c1111111-1111-1111-1111-111111111111", FunctionID: fraudDetect.FunctionID, VersionLabel: "v1"},
		{FunctionVersionID: "d1111111-1111-1111-1111-111111111111", FunctionID: embedText.FunctionID, VersionLabel: "v1"},
	}
	for _, v := range c.Versions {
		c.VersionsByFunction[v.FunctionID] = append(c.VersionsByFunction[v.FunctionID], v)
	}

	// Accounts. Weights skew chat-heavy acme and batch-heavy globex; initech
	// is embeddings-only in the synth's invocation mix; umbrella mixed.
	c.Accounts = []*Account{
		{AccountName: "acme", AccountDisplayName: "Acme Inc.", TrafficWeight: 1.0},
		{AccountName: "globex", AccountDisplayName: "Globex Corp.", TrafficWeight: 0.7},
		{AccountName: "initech", AccountDisplayName: "Initech LLC", TrafficWeight: 0.5},
		{AccountName: "umbrella", AccountDisplayName: "Umbrella Corp.", TrafficWeight: 0.4},
	}

	// Clusters.
	usWest := &Cluster{
		NVCAClusterName:  "us-west-2-a",
		NVCAClusterGroup: "gpu-heavy",
		InstanceType:     "NCP.GPU.A100_80GB_1x",
		NodeCount:        2,
		GPUsPerNode:      8,
	}
	usEast := &Cluster{
		NVCAClusterName:  "us-east-1-a",
		NVCAClusterGroup: "gpu-heavy",
		InstanceType:     "NCP.GPU.H100_80GB_1x",
		NodeCount:        2,
		GPUsPerNode:      4,
	}
	c.Clusters = []*Cluster{usWest, usEast}

	// Instances. one per (cluster × node × gpu).
	deviceIdx := 0
	for _, cl := range c.Clusters {
		modelName := "NVIDIA A100-SXM4-80GB"
		if cl.InstanceType == "NCP.GPU.H100_80GB_1x" {
			modelName = "NVIDIA H100-SXM5-80GB"
		}
		for node := 0; node < cl.NodeCount; node++ {
			for gpu := 0; gpu < cl.GPUsPerNode; gpu++ {
				inst := &Instance{
					NVCAClusterName: cl.NVCAClusterName,
					NodeIndex:       node,
					GPUIndex:        gpu,
					Device:          fmt.Sprintf("nvidia%d", gpu),
					Hostname:        fmt.Sprintf("%s-n%d", cl.NVCAClusterName, node),
					PCIBusID:        fmt.Sprintf("00000000:%02X:00.0", gpu),
					ModelName:       modelName,
				}
				c.Instances = append(c.Instances, inst)
				c.InstancesByCluster[cl.NVCAClusterName] = append(c.InstancesByCluster[cl.NVCAClusterName], inst)
				deviceIdx++
			}
		}
	}

	// Inference servers. One per (function_version × instance) where the
	// function is "running on" that cluster. For M1 we say every LLM-shaped
	// version runs on every instance; embed/fraud limited to one cluster.
	for _, v := range c.Versions {
		fn := findFunction(c.Functions, v.FunctionID)
		clusters := c.Clusters
		// Restrict latency-only / embed functions to one cluster to keep the
		// "active inference servers" panel honest (different shapes deploy
		// differently in real life).
		if fn.FunctionName == "fraud-detect" {
			clusters = []*Cluster{usEast}
		} else if fn.FunctionName == "embed-text" {
			clusters = []*Cluster{usWest}
		}
		for _, cl := range clusters {
			for _, inst := range c.InstancesByCluster[cl.NVCAClusterName] {
				srv := &InferenceServer{
					InferenceServerID: fmt.Sprintf("srv-%s-%s-%d-%d", v.FunctionVersionID[:8], cl.NVCAClusterName, inst.NodeIndex, inst.GPUIndex),
					FunctionVersionID: v.FunctionVersionID,
					FunctionID:        v.FunctionID,
					NVCAClusterName:   cl.NVCAClusterName,
					Device:            inst.Device,
					Model:             fn.FunctionName,
				}
				c.InferenceServers = append(c.InferenceServers, srv)
				c.ServersByVersion[v.FunctionVersionID] = append(c.ServersByVersion[v.FunctionVersionID], srv)
			}
		}
	}

	return c
}

func findFunction(funcs []*Function, id string) *Function {
	for _, f := range funcs {
		if f.FunctionID == id {
			return f
		}
	}
	return nil
}

// FunctionByID returns the seeded Function with the given function_id, or nil.
func (c *Catalog) FunctionByID(id string) *Function { return findFunction(c.Functions, id) }

// FunctionByName returns the seeded Function with the given function_name, or nil.
// Convenience for HTTP knob handlers that take user-friendly names.
func (c *Catalog) FunctionByName(name string) *Function {
	for _, f := range c.Functions {
		if f.FunctionName == name {
			return f
		}
	}
	return nil
}

// VersionByLabel returns the FunctionVersion with the given function_id and
// human-readable version label (e.g., "v1", "v2"), or nil.
func (c *Catalog) VersionByLabel(functionID, label string) *FunctionVersion {
	for _, v := range c.Versions {
		if v.FunctionID == functionID && v.VersionLabel == label {
			return v
		}
	}
	return nil
}
