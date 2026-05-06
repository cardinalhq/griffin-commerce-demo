// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ClientOpts configures a per-service polling client.
type ClientOpts struct {
	// URL is the control plane's base URL (e.g. http://controlplane:8086).
	// If empty, polling is skipped — Active() always returns nil.
	URL string

	// Service is the polling service's own service name. Knobs whose Service
	// field doesn't match this (or ServiceGlobal) are ignored — that's the
	// per-service dispatch rule.
	Service string

	// OnActivate fires once when a previously-nil knob becomes active, OR
	// when the active knob changes Key. Use it to spawn background
	// goroutines (gc-storm churn, memleak grower, etc.).
	OnActivate func(ctx context.Context, k *Knob)

	// OnClear fires once when the previously-active knob becomes nil, OR
	// when the active knob changes Key (the previous Knob is passed). Use
	// it to stop background goroutines.
	OnClear func(ctx context.Context, k *Knob)

	// PollInterval is the base polling cadence. Defaults to 1s. Each poll
	// adds ±200ms jitter so 6 services don't synchronize their requests.
	PollInterval time.Duration
}

// Client polls the control plane for the active knob and exposes it via
// Active() with lock-free reads (atomic.Pointer). Transition callbacks run
// under a mutex so background goroutines don't double-spawn or leak on
// rapid PUT/PUT/DELETE sequences.
type Client struct {
	opts ClientOpts

	active atomic.Pointer[Knob]

	mu    sync.Mutex // serializes transition callbacks
	httpc *http.Client
	rng   *rand.Rand
	rngMu sync.Mutex
}

// NewClient builds an unstarted client. Call Start to begin polling.
func NewClient(opts ClientOpts) *Client {
	if opts.PollInterval == 0 {
		opts.PollInterval = time.Second
	}
	return &Client{
		opts:  opts,
		httpc: &http.Client{Timeout: 2 * time.Second},
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Active returns the currently-active knob applicable to this service, or
// nil if none. Lock-free; safe to call from request hot paths.
func (c *Client) Active() *Knob {
	return c.active.Load()
}

// Start launches the polling goroutine. It returns immediately; polling
// stops when ctx is cancelled.
func (c *Client) Start(ctx context.Context) {
	if c.opts.URL == "" {
		slog.InfoContext(ctx, "faults: CONTROLPLANE_URL unset, polling disabled", "service", c.opts.Service)
		return
	}
	go c.pollLoop(ctx)
}

func (c *Client) pollLoop(ctx context.Context) {
	for {
		// Jitter ±20% of the base interval so distinct services don't
		// herd against the control plane.
		c.rngMu.Lock()
		jitter := time.Duration(c.rng.Int63n(int64(c.opts.PollInterval) / 5))
		c.rngMu.Unlock()
		dur := c.opts.PollInterval - c.opts.PollInterval/10 + jitter

		select {
		case <-ctx.Done():
			return
		case <-time.After(dur):
		}
		c.pollOnce(ctx)
	}
}

func (c *Client) pollOnce(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.opts.URL+"/admin/faults", nil)
	if err != nil {
		return
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		// Control plane down/unreachable — stay with last known state. The
		// alternative (clearing on every failed poll) would flap goroutines.
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	var body struct {
		Active *Knob `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return
	}
	c.applyTransition(ctx, body.Active)
}

// applyTransition implements the diff documented in the plan:
//
//	old=nil,  new=nil    → no-op
//	old=nil,  new=K      → Activate
//	old=K,    new=nil    → Clear(K)
//	old=K1,   new=K2 same key → swap pointer (reconfigure)
//	old=K1,   new=K2 diff key → Clear(K1), then Activate(K2)
//
// The dispatch filter (only act on knobs targeting this service or
// ServiceGlobal) runs first; a knob aimed at a different service appears
// to this client as nil.
func (c *Client) applyTransition(ctx context.Context, raw *Knob) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := raw
	if next != nil && next.Service != c.opts.Service && next.Service != ServiceGlobal {
		next = nil
	}

	prev := c.active.Load()

	switch {
	case prev == nil && next == nil:
		return
	case prev == nil && next != nil:
		c.active.Store(next)
		if c.opts.OnActivate != nil {
			c.opts.OnActivate(ctx, next)
		}
	case prev != nil && next == nil:
		c.active.Store(nil)
		if c.opts.OnClear != nil {
			c.opts.OnClear(ctx, prev)
		}
	case prev.Key == next.Key:
		// Same knob, possibly different params. Swap the pointer so
		// hot-path readers see the new params; no transition callback —
		// background goroutines that care about params should re-read
		// Active() each tick.
		c.active.Store(next)
	default:
		// Different knob entirely.
		c.active.Store(next)
		if c.opts.OnClear != nil {
			c.opts.OnClear(ctx, prev)
		}
		if c.opts.OnActivate != nil {
			c.opts.OnActivate(ctx, next)
		}
	}
}
