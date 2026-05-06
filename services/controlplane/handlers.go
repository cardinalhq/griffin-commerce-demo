// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package controlplane

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
)

// adminEnabled gates mutation endpoints. Off by default; the chaos overlay
// sets GRIFFIN_ADMIN_ENABLED=true. Reads (GET) are always allowed.
func adminEnabled() bool {
	return os.Getenv("GRIFFIN_ADMIN_ENABLED") == "true"
}

// RegisterRoutes wires the control plane's HTTP surface onto r.
func RegisterRoutes(r *mux.Router, s *state) {
	r.HandleFunc("/healthz", healthHandler).Methods("GET")
	r.HandleFunc("/admin/faults", getActiveHandler(s)).Methods("GET", "OPTIONS")
	r.HandleFunc("/admin/faults", putActiveHandler(s)).Methods("PUT", "OPTIONS")
	r.HandleFunc("/admin/faults", deleteActiveHandler(s)).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/admin/faults/catalog", getCatalogHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/admin/faults/events", eventsSSEHandler(s)).Methods("GET")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := common.WriteJSONResponse(w, common.HealthResponse{
		Status:    "healthy",
		Service:   "controlplane-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write health response", "error", err)
	}
}

// activeResponse is the body returned by GET /admin/faults.
type activeResponse struct {
	Active    *faults.Knob `json:"active"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

func getActiveHandler(s *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		k, ts := s.get()
		if err := common.WriteJSONResponse(w, activeResponse{Active: k, UpdatedAt: ts}, http.StatusOK); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write active response", "error", err)
		}
	}
}

func putActiveHandler(s *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationID := common.GetCorrelationID(r.Context())

		if !adminEnabled() {
			common.WriteErrorResponse(r.Context(), w,
				common.NewAppError("ADMIN_DISABLED", "Admin mutations disabled. Set GRIFFIN_ADMIN_ENABLED=true on the control plane."),
				http.StatusServiceUnavailable, correlationID)
			return
		}

		var k faults.Knob
		if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
			common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
			return
		}

		def, ok := lookupDefinition(k.Key)
		if !ok {
			common.WriteErrorResponse(r.Context(), w,
				common.NewAppError("UNKNOWN_KEY", fmt.Sprintf("Unknown knob key: %q", k.Key)),
				http.StatusBadRequest, correlationID)
			return
		}
		// Force Service / Kind from the catalog so clients can't fake
		// dispatch. Other params (probability, latency, status, target)
		// remain client-controlled.
		k.Service = def.Service
		k.Kind = def.Kind

		stored := s.put(&k)
		slog.InfoContext(r.Context(), "fault activated",
			"key", stored.Key, "service", stored.Service, "kind", stored.Kind,
			"target", stored.Target, "probability", stored.Probability,
			"latency_ms", stored.LatencyMs, "status_code", stored.StatusCode,
		)
		if err := common.WriteJSONResponse(w, stored, http.StatusOK); err != nil {
			slog.ErrorContext(r.Context(), "Failed to write put response", "error", err)
		}
	}
}

func deleteActiveHandler(s *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationID := common.GetCorrelationID(r.Context())
		if !adminEnabled() {
			common.WriteErrorResponse(r.Context(), w,
				common.NewAppError("ADMIN_DISABLED", "Admin mutations disabled."),
				http.StatusServiceUnavailable, correlationID)
			return
		}
		s.put(nil)
		slog.InfoContext(r.Context(), "fault cleared")
		w.WriteHeader(http.StatusNoContent)
	}
}

func getCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if err := common.WriteJSONResponse(w, KnobCatalog, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write catalog response", "error", err)
	}
}

// eventsSSEHandler streams the event log to the client. On connect it
// replays the in-memory ring buffer so a UI loading mid-session sees
// recent history, then forwards live events.
func eventsSSEHandler(s *state) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// Replay the buffer first so a fresh client sees history.
		for _, ev := range s.snapshotEvents() {
			writeSSE(w, ev)
		}
		flusher.Flush()

		ch := s.subscribe()
		defer s.unsubscribe(ch)

		ctx := r.Context()
		// Heartbeat every 15s so proxies don't reap an idle stream.
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				writeSSE(w, ev)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, ev Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
}
