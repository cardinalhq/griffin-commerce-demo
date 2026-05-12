// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"log/slog"
	"os"

	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
)

// Knob keys this service responds to. The demo presenter activates the
// dbaas.disk-full knob from the controlplane UI to trigger the scenario.
const (
	knobDiskFull = "dbaas.disk-full"
)

// faultsClient polls the controlplane for the active knob. nil when
// CONTROLPLANE_URL is unset (local dev / unit tests).
var faultsClient *faults.Client

// startFaultsClient registers this service with the controlplane and starts
// the polling loop. The dispatch flips per-instance flags on the fleet
// state so the metric callback reads them lock-free.
func startFaultsClient(ctx context.Context) {
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServiceDBaaS,
		OnActivate: func(ctx context.Context, k *faults.Knob) {
			applyKnob(ctx, k, true)
		},
		OnClear: func(ctx context.Context, k *faults.Knob) {
			applyKnob(ctx, k, false)
		},
	})
	faultsClient.Start(ctx)
}

// applyKnob is invoked on knob transitions. For dbaas.disk-full it flips
// the diskFullActive flag on the targeted instance(s). Target unset →
// applies to the default victim hdfc-prod-03.
func applyKnob(ctx context.Context, k *faults.Knob, active bool) {
	if k == nil {
		return
	}
	switch k.Key {
	case knobDiskFull:
		target := k.Target
		if target == "" {
			target = "hdfc-prod-03"
		}
		applied := 0
		for _, st := range fleetState {
			if st.Inst.DBID == target {
				st.diskFullActive.Store(active)
				applied++
			}
		}
		slog.InfoContext(ctx, "dbaas.disk-full knob transition",
			"active", active,
			"target", target,
			"applied", applied,
		)
	default:
		// Knobs for other services arrive here via ServiceGlobal — ignore
		// anything that isn't ours.
	}
}
