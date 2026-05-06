// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
)

// newTestServer returns a test HTTP server backed by a fresh state. By
// default GRIFFIN_ADMIN_ENABLED is set so PUT/DELETE are allowed.
func newTestServer(t *testing.T) (*httptest.Server, *state) {
	t.Helper()
	t.Setenv("GRIFFIN_ADMIN_ENABLED", "true")
	s := newState()
	r := mux.NewRouter()
	RegisterRoutes(r, s)
	return httptest.NewServer(r), s
}

func TestHandlers_RoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	// Initial GET — no active knob.
	resp, err := http.Get(srv.URL + "/admin/faults")
	if err != nil {
		t.Fatalf("initial GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial GET status = %d, want 200", resp.StatusCode)
	}
	var initial activeResponse
	if err := json.NewDecoder(resp.Body).Decode(&initial); err != nil {
		t.Fatalf("decode initial: %v", err)
	}
	if initial.Active != nil {
		t.Errorf("expected initial Active=nil, got %+v", initial.Active)
	}

	// PUT a known knob.
	body, _ := json.Marshal(faults.Knob{Key: "catalog.error", Target: "PROD-001", StatusCode: 503})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/admin/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", resp.StatusCode)
	}
	var stored faults.Knob
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	// Server must overwrite Service/Kind from the catalog regardless of
	// what the client sent, so dispatch can't be spoofed.
	if stored.Service != faults.ServiceCatalog {
		t.Errorf("stored.Service = %q, want %q", stored.Service, faults.ServiceCatalog)
	}
	if stored.Kind != faults.KindError {
		t.Errorf("stored.Kind = %q, want %q", stored.Kind, faults.KindError)
	}
	if stored.Target != "PROD-001" {
		t.Errorf("stored.Target = %q, want PROD-001", stored.Target)
	}
	if stored.StatusCode != 503 {
		t.Errorf("stored.StatusCode = %d, want 503", stored.StatusCode)
	}
	if stored.StartedAt.IsZero() {
		t.Errorf("StartedAt was not set")
	}

	// GET — active knob present.
	resp, err = http.Get(srv.URL + "/admin/faults")
	if err != nil {
		t.Fatalf("GET after PUT: %v", err)
	}
	defer resp.Body.Close()
	var got activeResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got.Active == nil || got.Active.Key != "catalog.error" {
		t.Fatalf("expected active=catalog.error, got %+v", got.Active)
	}

	// DELETE — clears.
	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/admin/faults", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}

	// GET — back to nil.
	resp, err = http.Get(srv.URL + "/admin/faults")
	if err != nil {
		t.Fatalf("GET after DELETE: %v", err)
	}
	defer resp.Body.Close()
	var afterDelete activeResponse
	if err := json.NewDecoder(resp.Body).Decode(&afterDelete); err != nil {
		t.Fatalf("decode get-after-delete: %v", err)
	}
	if afterDelete.Active != nil {
		t.Errorf("expected nil after delete, got %+v", afterDelete.Active)
	}
}

func TestHandlers_PutRejectsUnknownKey(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	body := []byte(`{"key":"definitely-not-a-real-knob"}`)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/admin/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown key, got %d", resp.StatusCode)
	}
}

func TestHandlers_AdminDisabledBlocksPutAndDelete(t *testing.T) {
	t.Setenv("GRIFFIN_ADMIN_ENABLED", "false")
	s := newState()
	r := mux.NewRouter()
	RegisterRoutes(r, s)
	srv := httptest.NewServer(r)
	defer srv.Close()

	body, _ := json.Marshal(faults.Knob{Key: "cart.error"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/admin/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("PUT with admin disabled status = %d, want 503", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/admin/faults", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("DELETE with admin disabled status = %d, want 503", resp.StatusCode)
	}

	// GET still works.
	resp, err = http.Get(srv.URL + "/admin/faults")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET status = %d, want 200", resp.StatusCode)
	}
}

func TestHandlers_CatalogReturnsKnownKnobs(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/admin/faults/catalog")
	if err != nil {
		t.Fatalf("GET catalog: %v", err)
	}
	defer resp.Body.Close()
	var defs []faults.KnobDefinition
	if err := json.NewDecoder(resp.Body).Decode(&defs); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(defs) < 10 {
		t.Errorf("catalog returned %d knobs, expected at least 10", len(defs))
	}

	wantKeys := []string{
		"catalog.error", "catalog.slow",
		"cart.error", "cart.outlier", "cart.poison-product",
		"payment.fail", "payment.gc-storm",
		"shipping.fail",
		"images.slow",
		"recs.memleak",
		"global.cpu-burn-traffic", "global.cpu-burn-bg",
	}
	have := map[string]bool{}
	for _, d := range defs {
		have[d.Key] = true
	}
	for _, k := range wantKeys {
		if !have[k] {
			t.Errorf("catalog missing knob key %q", k)
		}
	}
}

func TestHandlers_SSEReplayThenLive(t *testing.T) {
	srv, s := newTestServer(t)
	defer srv.Close()

	// Pre-populate one event so the replay path is exercised.
	s.put(&faults.Knob{Key: "cart.error", Service: faults.ServiceCart, Kind: faults.KindError})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/faults/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream prefix", got)
	}

	// Read bytes until we see the replayed activate event line.
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("read SSE: %v", err)
	}
	body := string(buf[:n])
	if !strings.Contains(body, "event: activate") {
		t.Errorf("SSE replay did not contain activate event: %q", body)
	}
}

func TestHandlers_Healthz(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", resp.StatusCode)
	}
}
