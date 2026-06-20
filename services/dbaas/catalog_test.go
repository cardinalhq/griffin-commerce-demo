// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogShape(t *testing.T) {
	c := NewCatalog()
	require.Len(t, c.Tenants, 6)
	require.Len(t, c.Clusters, 2)
	require.Len(t, c.Hosts, 4)
	require.Len(t, c.Datastores, 3)
	require.Len(t, c.VMs, 6) // 5 from spec §4.6 plus the acme noisy-neighbor
	require.Len(t, c.PGInstances, 4)
}

func TestEveryPGHasHostAndDatastore(t *testing.T) {
	c := NewCatalog()
	for _, pg := range c.PGInstances {
		require.NotNil(t, pg.VM, "pg %s has no VM", pg.Name)
		require.NotNil(t, pg.VM.Host, "pg %s VM has no host", pg.Name)
		require.NotNil(t, pg.VM.Datastore, "pg %s VM has no datastore", pg.Name)
		require.NotNil(t, pg.VM.Cluster, "pg %s VM has no cluster", pg.Name)
		require.NotNil(t, pg.Tenant, "pg %s has no tenant", pg.Name)
	}
}

func TestPrimaryAndAtRiskShareInfra(t *testing.T) {
	// The whole demo story (spec §22.4) depends on the primary impacted PG
	// and the at-risk PG sharing both ESXi host and datastore.
	c := NewCatalog()
	var primary, atRisk *VM
	for _, v := range c.VMs {
		switch v.IncidentRole {
		case rolePrimaryVM:
			primary = v
		case roleAtRiskVM:
			atRisk = v
		}
	}
	require.NotNil(t, primary, "no primary impacted VM in catalog")
	require.NotNil(t, atRisk, "no at-risk VM in catalog")
	require.Equal(t, primary.Host.ID, atRisk.Host.ID,
		"primary and at-risk VMs must share the degraded host")
	require.Equal(t, primary.Datastore.ID, atRisk.Datastore.ID,
		"primary and at-risk VMs must share the degraded datastore")
}

func TestDegradedDatastoreAndHostExist(t *testing.T) {
	c := NewCatalog()
	var degHost *Host
	var degDS *Datastore
	for _, h := range c.Hosts {
		if h.IncidentRole == roleDegradedHost {
			degHost = h
		}
	}
	for _, d := range c.Datastores {
		if d.IncidentRole == roleDegradedDS {
			degDS = d
		}
	}
	require.NotNil(t, degHost, "no degraded host in catalog")
	require.NotNil(t, degDS, "no degraded datastore in catalog")
}

func TestNoisyNeighborSharesHostWithPrimary(t *testing.T) {
	c := NewCatalog()
	var primary, noisy *VM
	for _, v := range c.VMs {
		switch v.IncidentRole {
		case rolePrimaryVM:
			primary = v
		case roleNoisyNeighbor:
			noisy = v
		}
	}
	require.NotNil(t, primary)
	require.NotNil(t, noisy)
	require.Equal(t, primary.Host.ID, noisy.Host.ID,
		"noisy neighbor must share the host with the primary impacted VM")
}

func TestVMsOnHostAndDatastore(t *testing.T) {
	c := NewCatalog()
	vmsOnHost1017 := c.VMsOnHost("host-1017")
	require.NotEmpty(t, vmsOnHost1017)
	vmsOnDS202 := c.VMsOnDatastore("datastore-202")
	require.NotEmpty(t, vmsOnDS202)
}

func TestPGForVM(t *testing.T) {
	c := NewCatalog()
	pg := c.PGForVM("vm-bajaj-pg-01")
	require.NotNil(t, pg)
	require.Equal(t, "pg-bajaj-01", pg.Name)
}
