// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// applyTransition is the load-bearing logic of the client. We test it
// directly (no HTTP) because polling cadence + jitter would make the
// HTTP path slow and flaky. A separate "polls and decodes" test below
// exercises the HTTP read path.

type recorder struct {
	mu          sync.Mutex
	activates   []*Knob
	clears      []*Knob
}

func (r *recorder) onActivate(_ context.Context, k *Knob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activates = append(r.activates, k)
}

func (r *recorder) onClear(_ context.Context, k *Knob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clears = append(r.clears, k)
}

func (r *recorder) snapshot() (acts, clears []*Knob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := append([]*Knob(nil), r.activates...)
	c := append([]*Knob(nil), r.clears...)
	return a, c
}

func newClientWithRecorder(service string) (*Client, *recorder) {
	rec := &recorder{}
	c := NewClient(ClientOpts{
		URL:        "http://example.invalid",
		Service:    service,
		OnActivate: rec.onActivate,
		OnClear:    rec.onClear,
	})
	return c, rec
}

func TestClient_TransitionNilToKnob(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	c.applyTransition(context.Background(), &Knob{Key: "cart.error", Service: ServiceCart, Kind: KindError})

	acts, clears := rec.snapshot()
	if len(acts) != 1 || acts[0].Key != "cart.error" {
		t.Errorf("expected one activate for cart.error, got %+v", acts)
	}
	if len(clears) != 0 {
		t.Errorf("expected no clears, got %+v", clears)
	}
	if c.Active() == nil {
		t.Errorf("Active() is nil after transition")
	}
}

func TestClient_TransitionKnobToNil(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	k := &Knob{Key: "cart.error", Service: ServiceCart, Kind: KindError}
	c.applyTransition(context.Background(), k)
	c.applyTransition(context.Background(), nil)

	acts, clears := rec.snapshot()
	if len(acts) != 1 {
		t.Errorf("expected 1 activate, got %d", len(acts))
	}
	if len(clears) != 1 || clears[0].Key != "cart.error" {
		t.Errorf("expected one clear for cart.error, got %+v", clears)
	}
	if c.Active() != nil {
		t.Errorf("expected Active()=nil after clear")
	}
}

func TestClient_TransitionDifferentKnobsClearsOldThenActivatesNew(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	k1 := &Knob{Key: "cart.error", Service: ServiceCart, Kind: KindError}
	k2 := &Knob{Key: "cart.outlier", Service: ServiceCart, Kind: KindOutlier}
	c.applyTransition(context.Background(), k1)
	c.applyTransition(context.Background(), k2)

	acts, clears := rec.snapshot()
	if len(acts) != 2 || acts[0].Key != "cart.error" || acts[1].Key != "cart.outlier" {
		t.Errorf("activates unexpected: %+v", acts)
	}
	if len(clears) != 1 || clears[0].Key != "cart.error" {
		t.Errorf("clears unexpected: %+v", clears)
	}
}

func TestClient_TransitionSameKeyReconfiguresWithoutCallbacks(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	k1 := &Knob{Key: "cart.outlier", Service: ServiceCart, Kind: KindOutlier, Probability: 0.05, LatencyMs: 30000}
	k2 := &Knob{Key: "cart.outlier", Service: ServiceCart, Kind: KindOutlier, Probability: 0.10, LatencyMs: 60000}
	c.applyTransition(context.Background(), k1)
	c.applyTransition(context.Background(), k2)

	acts, clears := rec.snapshot()
	if len(acts) != 1 {
		t.Errorf("same-key transition should not re-fire OnActivate; got %d activations", len(acts))
	}
	if len(clears) != 0 {
		t.Errorf("same-key transition should not fire OnClear; got %d clears", len(clears))
	}
	if got := c.Active(); got == nil || got.Probability != 0.10 {
		t.Errorf("Active() didn't pick up new params: %+v", got)
	}
}

func TestClient_DispatchFiltersOtherServices(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	other := &Knob{Key: "catalog.error", Service: ServiceCatalog, Kind: KindError}
	c.applyTransition(context.Background(), other)

	acts, clears := rec.snapshot()
	if len(acts) != 0 || len(clears) != 0 {
		t.Errorf("knob targeting another service should be ignored; got acts=%+v clears=%+v", acts, clears)
	}
	if c.Active() != nil {
		t.Errorf("Active() should remain nil for foreign-service knob")
	}
}

func TestClient_DispatchAcceptsGlobal(t *testing.T) {
	c, rec := newClientWithRecorder(ServiceCart)
	g := &Knob{Key: "global.cpu-burn-traffic", Service: ServiceGlobal, Kind: KindCPUBurn, LatencyMs: 50}
	c.applyTransition(context.Background(), g)

	acts, _ := rec.snapshot()
	if len(acts) != 1 {
		t.Errorf("global knob should activate on every service; got acts=%+v", acts)
	}
}

func TestClient_DispatchTransitionAwayFromForeignKnobIsNoOp(t *testing.T) {
	// If the cart client previously saw nil and the control plane returns a
	// catalog-targeted knob, the cart client should still see "no active
	// knob" and emit no callbacks. Then if cleared, no spurious clear fires.
	c, rec := newClientWithRecorder(ServiceCart)
	c.applyTransition(context.Background(), &Knob{Key: "catalog.error", Service: ServiceCatalog, Kind: KindError})
	c.applyTransition(context.Background(), nil)

	acts, clears := rec.snapshot()
	if len(acts) != 0 {
		t.Errorf("foreign knob should not have activated; got %+v", acts)
	}
	if len(clears) != 0 {
		t.Errorf("clearing a never-seen knob should not call OnClear; got %+v", clears)
	}
}

// TestClient_PollOncePicksUpStateFromHTTP exercises the HTTP read path
// against a local stub so we know the JSON shape on the wire matches what
// the control plane emits.
func TestClient_PollOncePicksUpStateFromHTTP(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body := map[string]any{
			"active":    map[string]any{"key": "cart.error", "service": "cart", "kind": "error"},
			"updatedAt": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	rec := &recorder{}
	c := NewClient(ClientOpts{
		URL:        srv.URL,
		Service:    ServiceCart,
		OnActivate: rec.onActivate,
		OnClear:    rec.onClear,
	})
	c.pollOnce(context.Background())

	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 HTTP call, got %d", got)
	}
	acts, _ := rec.snapshot()
	if len(acts) != 1 || acts[0].Key != "cart.error" {
		t.Errorf("expected activate after poll; got %+v", acts)
	}
}

func TestClient_PollOnceTolerates5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	rec := &recorder{}
	c := NewClient(ClientOpts{
		URL:        srv.URL,
		Service:    ServiceCart,
		OnActivate: rec.onActivate,
		OnClear:    rec.onClear,
	})
	// Prime with an active knob, then poll: a 5xx must NOT clear it.
	c.applyTransition(context.Background(), &Knob{Key: "cart.error", Service: ServiceCart, Kind: KindError})
	c.pollOnce(context.Background())

	_, clears := rec.snapshot()
	if len(clears) != 0 {
		t.Errorf("5xx poll should not flap state; got clears=%+v", clears)
	}
	if c.Active() == nil {
		t.Errorf("Active() should still be set after 5xx poll")
	}
}
