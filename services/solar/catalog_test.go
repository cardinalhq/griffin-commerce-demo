// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogShape(t *testing.T) {
	c := NewCatalog()
	require.Equal(t, "khavda-1", c.Site)
	require.Len(t, c.Offtakers, 3)
	require.Len(t, c.MVCompounds, 3)
	require.Len(t, c.Transformers, 4)
	require.Len(t, c.Blocks, 6)
	require.Len(t, c.Stations, 24)
	require.Len(t, c.Inverters, 96)
	require.Len(t, c.Trackers, 6)
	require.Len(t, c.MetStations, 12)
	require.NotNil(t, c.Substation)
}

func TestSiblingTransformersShareCompound(t *testing.T) {
	// The blast-radius story (§22) depends on T-04-A and T-04-B sharing
	// mvc-04. If a rename or refactor breaks this, the demo is dead.
	c := NewCatalog()
	siblings := c.TransformersInCompound(mvCompound04)
	require.Len(t, siblings, 2)
	ids := []string{siblings[0].ID, siblings[1].ID}
	require.Contains(t, ids, mvTrafoT04A)
	require.Contains(t, ids, mvTrafoT04B)
}

func TestPrimaryAndAtRiskTransformerRoles(t *testing.T) {
	c := NewCatalog()
	var primary, atRisk *MVTransformer
	for _, mt := range c.Transformers {
		switch mt.IncidentRole {
		case rolePrimaryTransformer:
			primary = mt
		case roleAtRiskTransformer:
			atRisk = mt
		}
	}
	require.NotNil(t, primary)
	require.NotNil(t, atRisk)
	require.Equal(t, primary.Compound.ID, atRisk.Compound.ID,
		"primary and at-risk transformers must share the degraded compound")
}

func TestBlocksWiredToTransformers(t *testing.T) {
	c := NewCatalog()
	onT04A := c.BlocksOnTransformer(mvTrafoT04A)
	require.Len(t, onT04A, 2)
	onT04B := c.BlocksOnTransformer(mvTrafoT04B)
	require.Len(t, onT04B, 2)
	primaryBlockIDs := []string{onT04A[0].ID, onT04A[1].ID}
	require.Contains(t, primaryBlockIDs, "block-04")
	require.Contains(t, primaryBlockIDs, "block-12")
	atRiskBlockIDs := []string{onT04B[0].ID, onT04B[1].ID}
	require.Contains(t, atRiskBlockIDs, "block-06")
	require.Contains(t, atRiskBlockIDs, "block-14")
}

func TestEveryBlockHasOfftakerAndTransformer(t *testing.T) {
	c := NewCatalog()
	for _, b := range c.Blocks {
		require.NotNil(t, b.Offtaker, "block %s has no offtaker", b.ID)
		require.NotNil(t, b.Transformer, "block %s has no transformer", b.ID)
		require.NotNil(t, b.Transformer.Compound, "block %s transformer has no compound", b.ID)
	}
}

func TestInverterCoolingTargetExists(t *testing.T) {
	// inverter_cooling_fault profile targets INV-08-02-03.
	c := NewCatalog()
	for _, inv := range c.Inverters {
		if inv.ID == "INV-08-02-03" {
			require.Equal(t, rolePrimaryInverter, inv.IncidentRole)
			require.NotNil(t, inv.Station)
			require.Equal(t, "IS-08-02", inv.Station.ID)
			return
		}
	}
	t.Fatalf("INV-08-02-03 not found in catalog")
}

func TestInvertersOnTransformer(t *testing.T) {
	c := NewCatalog()
	// 2 blocks × 4 stations × 4 inverters = 32 inverters on T-04-A.
	require.Len(t, c.InvertersOnTransformer(mvTrafoT04A), 32)
}

func TestBlocksForOfftaker(t *testing.T) {
	c := NewCatalog()
	require.Len(t, c.BlocksForOfftaker(offtakerSECI), 2)   // block-04 + block-12
	require.Len(t, c.BlocksForOfftaker(offtakerGUVNL), 3)  // block-06 + block-08 + block-14
	require.Len(t, c.BlocksForOfftaker(offtakerAdaniM), 1) // block-10
}
