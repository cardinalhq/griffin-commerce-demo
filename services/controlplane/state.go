// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package controlplane

import (
	"sync"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
)

// EventType discriminates SSE event-stream entries.
const (
	EventActivate = "activate"
	EventClear    = "clear"
)

// Event is one entry in the control plane's event log / SSE stream.
type Event struct {
	Type     string       `json:"type"`
	Knob     *faults.Knob `json:"knob,omitempty"`
	Previous *faults.Knob `json:"previous,omitempty"`
	At       time.Time    `json:"at"`
}

// eventsRingSize bounds the in-memory event log so a long-running control
// plane doesn't grow unbounded. SSE clients see this buffer replayed on
// connect, then live events.
const eventsRingSize = 100

// state owns all mutable control-plane data. Single-replica by design;
// horizontal scale would require externalizing this (Redis etc.).
type state struct {
	mu        sync.RWMutex
	active    *faults.Knob
	updatedAt time.Time

	eventsMu sync.Mutex
	events   []Event

	subsMu sync.Mutex
	subs   map[chan Event]struct{}
}

func newState() *state {
	return &state{
		updatedAt: time.Now(),
		events:    make([]Event, 0, eventsRingSize),
		subs:      make(map[chan Event]struct{}),
	}
}

// get returns a snapshot of the active knob and the timestamp of the last
// mutation. The returned *Knob is the live pointer — callers must treat
// it as read-only.
func (s *state) get() (*faults.Knob, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active, s.updatedAt
}

// put atomically replaces the active knob (or clears it when k is nil).
// StartedAt is overwritten with the server's wall clock so clients can't
// spoof activation time. Returns the stored knob (nil on clear).
func (s *state) put(k *faults.Knob) *faults.Knob {
	now := time.Now()

	s.mu.Lock()
	prev := s.active
	if k != nil {
		// Defensive copy so callers can't mutate stored state via their
		// own pointer.
		stored := *k
		stored.StartedAt = now
		s.active = &stored
	} else {
		s.active = nil
	}
	s.updatedAt = now
	cur := s.active
	s.mu.Unlock()

	var ev Event
	if cur != nil {
		ev = Event{Type: EventActivate, Knob: cur, At: now, Previous: prev}
	} else {
		ev = Event{Type: EventClear, Previous: prev, At: now}
	}
	s.recordEvent(ev)
	s.broadcast(ev)
	return cur
}

func (s *state) recordEvent(ev Event) {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	if len(s.events) >= eventsRingSize {
		// Drop oldest. copy+truncate avoids growing capacity unboundedly.
		copy(s.events, s.events[1:])
		s.events = s.events[:len(s.events)-1]
	}
	s.events = append(s.events, ev)
}

// snapshotEvents returns a copy of the ring buffer for SSE replay.
func (s *state) snapshotEvents() []Event {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// subscribe registers a channel for live event delivery. The caller must
// invoke unsubscribe to release the slot. The channel is buffered (16)
// and broadcast drops on full, so a stuck subscriber can't stall others.
func (s *state) subscribe() chan Event {
	ch := make(chan Event, 16)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()
	return ch
}

func (s *state) unsubscribe(ch chan Event) {
	s.subsMu.Lock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	s.subsMu.Unlock()
}

func (s *state) broadcast(ev Event) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
			// Slow subscriber — drop; UI will recover via the next
			// state snapshot fetch.
		}
	}
}
