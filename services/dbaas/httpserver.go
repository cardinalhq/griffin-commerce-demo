// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"
)

// httpserver.go exposes a small local HTTP API the demo operator drives
// from a kubectl port-forward to flip failure profiles. It deliberately
// has no auth — bind to a ClusterIP-only Service and access via
// `kubectl port-forward`.

const defaultFaultPort = "9999"

func faultPort() string {
	if v := os.Getenv("DBAAS_FAULT_PORT"); v != "" {
		return v
	}
	return defaultFaultPort
}

// StartHTTPServer binds the fault endpoint on :PORT and returns. The
// server is shut down when ctx is cancelled.
func StartHTTPServer(ctx context.Context, sc *Scenario) {
	mux := http.NewServeMux()
	mux.HandleFunc("/faults/activate", activateHandler(sc))
	mux.HandleFunc("/faults/clear", clearHandler(sc))
	mux.HandleFunc("/faults/status", statusHandler(sc))
	mux.HandleFunc("/faults/profiles", profilesHandler(sc))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":" + faultPort()
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.InfoContext(ctx, "DBaaS fault HTTP server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "fault server error", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

func activateHandler(sc *Scenario) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
			return
		}
		id := r.URL.Query().Get("profile")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing ?profile= parameter"})
			return
		}
		prev := sc.ActiveProfileID()
		start, err := sc.Activate(id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":          err.Error(),
				"known_profiles": sortedProfiles(sc),
				"active_before":  prev,
			})
			return
		}
		p, _ := sc.Lookup(id)
		writeJSON(w, http.StatusOK, map[string]any{
			"active":           id,
			"previous":         prev,
			"started_at":       start.UTC().Format(time.RFC3339Nano),
			"duration_minutes": p.Duration.Minutes(),
		})
	}
}

func clearHandler(sc *Scenario) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
			return
		}
		prev := sc.Clear()
		writeJSON(w, http.StatusOK, map[string]any{
			"cleared":  prev != "",
			"previous": prev,
		})
	}
}

func statusHandler(sc *Scenario) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		id, start, elapsed := sc.Status()
		resp := map[string]any{
			"active": id,
		}
		if id != "" {
			p, _ := sc.Lookup(id)
			resp["started_at"] = start.UTC().Format(time.RFC3339Nano)
			resp["elapsed_seconds"] = int(elapsed.Seconds())
			resp["duration_minutes"] = p.Duration.Minutes()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func profilesHandler(sc *Scenario) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"profiles": sortedProfiles(sc),
		})
	}
}

func sortedProfiles(sc *Scenario) []string {
	ids := sc.ProfileIDs()
	sort.Strings(ids)
	return ids
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
