// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package faults provides the runtime fault-injection knob model and the
// per-service polling client that consumes knob state from the central
// control plane.
package faults

import "time"

// Service identifiers. A Knob's Service decides which polling clients act on
// it; clients ignore knobs whose Service doesn't match their own — except
// ServiceGlobal, which every client applies.
const (
	ServiceCatalog         = "catalog"
	ServiceCart            = "cart"
	ServicePayment         = "payment"
	ServiceShipping        = "shipping"
	ServiceImages          = "images"
	ServiceRecommendations = "recs"
	ServiceGlobal          = "global"
	ServiceLoadgen         = "loadgen"
)

// Kind identifiers describe the shape of the fault, independent of which
// service it targets. Site-specific behavior at hook points keys off (Key,
// Kind) — Kind alone tells the generic middleware how to behave.
const (
	KindError   = "error"
	KindSlow    = "slow"
	KindOutlier = "outlier"
	KindMemleak = "memleak"
	KindCPUBurn = "cpuburn"
	KindGCStorm = "gcstorm"
	KindFlood   = "flood"
)

// Knob is a single fault-injection configuration. The control plane holds at
// most one active Knob at any time. omitempty on numeric fields keeps GETs
// readable for knobs that don't use them.
type Knob struct {
	Key         string    `json:"key"`
	Service     string    `json:"service"`
	Kind        string    `json:"kind"`
	Probability float64   `json:"probability,omitempty"`
	LatencyMs   int       `json:"latencyMs,omitempty"`
	StatusCode  int       `json:"statusCode,omitempty"`
	Target      string    `json:"target,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
}

// ParamSpec describes a single tunable parameter on a knob.
// Type ∈ {"int","float","string"}.
type ParamSpec struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Min         float64 `json:"min,omitempty"`
	Max         float64 `json:"max,omitempty"`
	Default     any     `json:"default,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
}

// KnobDefinition is the catalog entry for a knob. The control plane returns
// these from GET /admin/faults/catalog so the UI can render param inputs
// without hard-coding what each knob does.
type KnobDefinition struct {
	Key         string      `json:"key"`
	Service     string      `json:"service"`
	Kind        string      `json:"kind"`
	Description string      `json:"description"`
	Params      []ParamSpec `json:"params"`
	Guidance    string      `json:"guidance,omitempty"`
}
