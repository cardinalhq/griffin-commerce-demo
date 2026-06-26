// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"
)

// httpserver.go mirrors services/dbaas/httpserver.go: a small local HTTP
// API on :9998 the demo operator drives via kubectl port-forward to
// activate/clear chaos knobs. No auth — bind to ClusterIP only.

const defaultFaultPort = "9998"

func faultPort() string {
	if v := os.Getenv("NVCF_FAULT_PORT"); v != "" {
		return v
	}
	return defaultFaultPort
}

// StartHTTPServer binds the fault endpoint and returns. Shuts down when ctx ends.
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
		slog.InfoContext(ctx, "NVCF fault HTTP server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "NVCF fault server error", "err", err)
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
		// Collect non-profile query params as resolve args.
		args := make(map[string]string)
		for k, v := range r.URL.Query() {
			if k == "profile" || len(v) == 0 {
				continue
			}
			args[k] = v[0]
		}
		prev := sc.ActiveProfileID()
		start, err := sc.Activate(id, args)
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
			"active":        id,
			"previous":      prev,
			"started_at":    start.UTC().Format(time.RFC3339Nano),
			"duration_secs": int(p.Duration.Seconds()),
			"args":          args,
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
		id, start, elapsed, args := sc.Status()
		resp := map[string]any{
			"active": id,
		}
		if id != "" {
			p, _ := sc.Lookup(id)
			resp["started_at"] = start.UTC().Format(time.RFC3339Nano)
			resp["elapsed_seconds"] = int(elapsed.Seconds())
			resp["duration_secs"] = int(p.Duration.Seconds())
			resp["args"] = args
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
