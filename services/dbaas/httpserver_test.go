// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActivateHandlerValid(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodPost,
		"/faults/activate?profile="+ProfileDatastoreInfra, nil)
	w := httptest.NewRecorder()
	activateHandler(sc)(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, ProfileDatastoreInfra, body["active"])
	require.NotEmpty(t, body["started_at"])
	require.Equal(t, ProfileDatastoreInfra, sc.ActiveProfileID())
}

func TestActivateHandlerUnknown(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodPost, "/faults/activate?profile=garbage", nil)
	w := httptest.NewRecorder()
	activateHandler(sc)(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, sc.ActiveProfileID())
}

func TestActivateHandlerMissingProfile(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodPost, "/faults/activate", nil)
	w := httptest.NewRecorder()
	activateHandler(sc)(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestActivateHandlerGetRejected(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodGet, "/faults/activate?profile="+ProfileDiskSaturation, nil)
	w := httptest.NewRecorder()
	activateHandler(sc)(w, r)
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestActivateReplacesPrevious(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate(ProfileDiskSaturation)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/faults/activate?profile="+ProfileCPUReady, nil)
	w := httptest.NewRecorder()
	activateHandler(sc)(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, ProfileCPUReady, body["active"])
	require.Equal(t, ProfileDiskSaturation, body["previous"])
}

func TestClearHandler(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate(ProfileDiskSaturation)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/faults/clear", nil)
	w := httptest.NewRecorder()
	clearHandler(sc)(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Equal(t, ProfileDiskSaturation, body["previous"])
	require.Equal(t, true, body["cleared"])
	require.Empty(t, sc.ActiveProfileID())
}

func TestStatusHandler(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodGet, "/faults/status", nil)
	w := httptest.NewRecorder()
	statusHandler(sc)(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	body := strings.TrimSpace(w.Body.String())
	require.Contains(t, body, `"active":""`)

	_, err := sc.Activate(ProfileMemoryPressure)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	statusHandler(sc)(w, r)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, ProfileMemoryPressure, resp["active"])
}

func TestProfilesHandler(t *testing.T) {
	sc := NewScenario()
	r := httptest.NewRequest(http.MethodGet, "/faults/profiles", nil)
	w := httptest.NewRecorder()
	profilesHandler(sc)(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp["profiles"], 4)
}
