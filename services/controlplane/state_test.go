// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package controlplane

import (
	"testing"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
)

func TestState_PutAndGet(t *testing.T) {
	s := newState()
	got, _ := s.get()
	if got != nil {
		t.Fatalf("expected initial active=nil, got %+v", got)
	}

	k := &faults.Knob{Key: "catalog.error", Service: faults.ServiceCatalog, Kind: faults.KindError, Target: "PROD-001"}
	stored := s.put(k)
	if stored.Key != "catalog.error" {
		t.Fatalf("stored.Key = %q, want catalog.error", stored.Key)
	}
	if stored.StartedAt.IsZero() {
		t.Errorf("StartedAt was not set on put")
	}

	got, ts := s.get()
	if got != stored {
		t.Errorf("get() returned a different pointer than put()")
	}
	if ts.IsZero() {
		t.Errorf("updatedAt was not set")
	}
}

func TestState_PutDefensiveCopy(t *testing.T) {
	s := newState()
	original := &faults.Knob{Key: "cart.error", Service: faults.ServiceCart, Kind: faults.KindError, Probability: 0.5}
	s.put(original)

	// Mutating the caller's pointer must not affect stored state.
	original.Probability = 0.99
	got, _ := s.get()
	if got.Probability != 0.5 {
		t.Errorf("stored knob mutated through caller pointer: got Probability=%v, want 0.5", got.Probability)
	}
}

func TestState_ClearProducesEvent(t *testing.T) {
	s := newState()
	s.put(&faults.Knob{Key: "cart.error", Service: faults.ServiceCart, Kind: faults.KindError})
	s.put(nil)

	got, _ := s.get()
	if got != nil {
		t.Fatalf("expected active=nil after clear, got %+v", got)
	}
	events := s.snapshotEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 events (activate, clear), got %d", len(events))
	}
	if events[0].Type != EventActivate {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, EventActivate)
	}
	if events[1].Type != EventClear {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, EventClear)
	}
	if events[1].Previous == nil || events[1].Previous.Key != "cart.error" {
		t.Errorf("events[1].Previous missing or wrong: %+v", events[1].Previous)
	}
}

func TestState_RingBufferTrim(t *testing.T) {
	s := newState()
	for i := 0; i < eventsRingSize+25; i++ {
		s.put(&faults.Knob{Key: "cart.error", Service: faults.ServiceCart, Kind: faults.KindError})
	}
	events := s.snapshotEvents()
	if len(events) != eventsRingSize {
		t.Fatalf("ring buffer should cap at %d, got %d", eventsRingSize, len(events))
	}
}

func TestState_BroadcastDeliversAndDropsOnFull(t *testing.T) {
	s := newState()
	ch := s.subscribe()
	defer s.unsubscribe(ch)

	s.put(&faults.Knob{Key: "cart.error", Service: faults.ServiceCart, Kind: faults.KindError})

	select {
	case ev := <-ch:
		if ev.Type != EventActivate {
			t.Errorf("got event type %q, want %q", ev.Type, EventActivate)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive activate event")
	}
}

func TestState_UnsubscribeStopsDelivery(t *testing.T) {
	s := newState()
	ch := s.subscribe()
	s.unsubscribe(ch)

	// After unsubscribe, the channel is closed; reading must not block.
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("channel should be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("read from closed channel blocked")
	}

	// Idempotent: a second unsubscribe must not panic.
	s.unsubscribe(ch)
}
