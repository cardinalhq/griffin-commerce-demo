// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"log/slog"
	"os"
	"time"

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
// the diskFullActive flag on the targeted instance(s) AND drives the
// storage-ramp state machine in load.go via diskFullActivatedAt /
// postExpansion. Target unset → applies to the default victim hdfc-prod-03.
//
// activatedAt uses the controlplane's Knob.StartedAt rather than time.Now()
// so that if the pod restarts mid-scenario, polling picks up the still-
// active knob and resumes the ramp from the original activation timestamp
// (no visual reset on pod replacement).
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
		startedAt := k.StartedAt
		if startedAt.IsZero() {
			startedAt = time.Now()
		}
		applied := 0
		for _, st := range fleetState {
			if st.Inst.DBID == target {
				st.diskFullActive.Store(active)
				if active {
					t := startedAt
					st.diskFullActivatedAt.Store(&t)
					st.postExpansion.Store(false)
				} else {
					st.diskFullActivatedAt.Store(nil)
					st.postExpansion.Store(true)
				}
				applied++
			}
		}
		slog.InfoContext(ctx, "dbaas.disk-full knob transition",
			"active", active,
			"target", target,
			"activated_at", startedAt,
			"applied", applied,
		)
	default:
		// Knobs for other services arrive here via ServiceGlobal — ignore
		// anything that isn't ours.
	}
}
