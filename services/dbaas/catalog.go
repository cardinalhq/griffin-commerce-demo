// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

// catalog.go contains the static entity catalog from the Airtel demo spec
// (airtel_postgres_vmware_telemetry_spec.md §4). Every metric and log walks
// these pointers so the correlation join keys (tenant_id, vm_uuid,
// esxi_host_id, datastore_id) line up across all signals.

const (
	regionDelhi1 = "airtel-delhi-1"
	azA          = "az-a"
	azB          = "az-b"

	clusterDelhiA = "cluster-delhi-prod-a"
	clusterDelhiB = "cluster-delhi-prod-b"

	vcenterName     = "vcenter-delhi-01.airtelcloud.local"
	vcenterID       = "vc-delhi-01"
	datacenterDelhi = "dc-delhi-01"

	roleNone          = ""
	rolePrimaryVM     = "primary_impacted_vm"
	roleAtRiskVM      = "at_risk_vm"
	roleNoisyNeighbor = "noisy_neighbor"
	roleDegradedHost  = "degraded_host"
	roleDegradedDS    = "degraded_datastore"
	rolePrimaryPG     = "primary_impacted_pg"
	roleAtRiskPG      = "at_risk_pg"
)

type Tenant struct {
	TenantID     string
	CustomerID   string
	CustomerName string
	AccountID    string
	Tier         string
	Environment  string
	Region       string
	AZ           string
	SLOName      string
	SLOTargetMs  float64
	state        *tenantState
}

type Cluster struct {
	Name       string
	ID         string
	Datacenter string
	Region     string
	AZ         string
}

type Host struct {
	Name         string
	ID           string
	Cluster      *Cluster
	RackID       string
	CPUSockets   int
	CPUCores     int
	MemoryGiB    int
	IncidentRole string
	state        *hostState
}

type Datastore struct {
	Name         string
	ID           string
	Type         string
	StorageArray string
	ServiceTier  string
	CapacityGiB  int64
	IncidentRole string
	state        *datastoreState
}

type VM struct {
	Name         string
	UUID         string
	MOID         string
	Tenant       *Tenant
	WorkloadRole string
	OS           string
	VCPU         int
	MemoryGiB    int
	Host         *Host
	Datastore    *Datastore
	Cluster      *Cluster
	Region       string
	AZ           string
	IncidentRole string
	state        *vmState
}

type PGInstance struct {
	Name         string
	Cluster      string
	Role         string
	Version      string
	Database     string
	Port         int
	Tenant       *Tenant
	VM           *VM
	ServiceTier  string
	SLOName      string
	SLOTargetMs  float64
	IncidentRole string
	state        *pgState
}

type VCenter struct {
	Name       string
	ID         string
	Datacenter string
	Region     string
}

type Catalog struct {
	Tenants     []*Tenant
	VCenter     *VCenter
	Clusters    []*Cluster
	Hosts       []*Host
	Datastores  []*Datastore
	VMs         []*VM
	PGInstances []*PGInstance
}

// NewCatalog returns the spec §4 entity catalog with all cross-links resolved.
// Per-entity state is allocated and seeded.
func NewCatalog() *Catalog {
	c := &Catalog{
		VCenter: &VCenter{
			Name:       vcenterName,
			ID:         vcenterID,
			Datacenter: datacenterDelhi,
			Region:     regionDelhi1,
		},
	}

	tenants := []*Tenant{
		{TenantID: "tenant_bajaj_finance", CustomerID: "bajaj_finance", CustomerName: "Bajaj Finance",
			AccountID: "airtel-acct-10021", Tier: "gold", Environment: "prod",
			Region: regionDelhi1, AZ: azA, SLOName: "postgres_p95_latency", SLOTargetMs: 120},
		{TenantID: "tenant_indigo_ops", CustomerID: "indigo_ops", CustomerName: "IndiGo Operations",
			AccountID: "airtel-acct-10044", Tier: "gold", Environment: "prod",
			Region: regionDelhi1, AZ: azA, SLOName: "postgres_p95_latency", SLOTargetMs: 120},
		{TenantID: "tenant_apollo_health", CustomerID: "apollo_health", CustomerName: "Apollo Health",
			AccountID: "airtel-acct-10077", Tier: "silver", Environment: "prod",
			Region: regionDelhi1, AZ: azB, SLOName: "postgres_p95_latency", SLOTargetMs: 180},
		{TenantID: "tenant_acme_retail", CustomerID: "acme_retail", CustomerName: "ACME Retail",
			AccountID: "airtel-acct-10089", Tier: "silver", Environment: "prod",
			Region: regionDelhi1, AZ: azA, SLOName: "postgres_p95_latency", SLOTargetMs: 180},
		{TenantID: "tenant_mahindra_auto", CustomerID: "mahindra_auto", CustomerName: "Mahindra Auto",
			AccountID: "airtel-acct-10111", Tier: "gold", Environment: "prod",
			Region: regionDelhi1, AZ: azB, SLOName: "postgres_p95_latency", SLOTargetMs: 120},
		{TenantID: "tenant_hdfc_life", CustomerID: "hdfc_life", CustomerName: "HDFC Life",
			AccountID: "airtel-acct-10142", Tier: "gold", Environment: "prod",
			Region: regionDelhi1, AZ: azA, SLOName: "postgres_p95_latency", SLOTargetMs: 120},
	}
	for _, t := range tenants {
		t.state = newTenantState(t)
	}
	c.Tenants = tenants
	tenantByID := map[string]*Tenant{}
	for _, t := range tenants {
		tenantByID[t.TenantID] = t
	}

	clusters := []*Cluster{
		{Name: clusterDelhiA, ID: "domain-c101", Datacenter: datacenterDelhi, Region: regionDelhi1, AZ: azA},
		{Name: clusterDelhiB, ID: "domain-c102", Datacenter: datacenterDelhi, Region: regionDelhi1, AZ: azB},
	}
	c.Clusters = clusters
	clusterByName := map[string]*Cluster{}
	for _, cl := range clusters {
		clusterByName[cl.Name] = cl
	}

	hosts := []*Host{
		{Name: "esx-delhi-a-15.airtelcloud.local", ID: "host-1015", Cluster: clusterByName[clusterDelhiA],
			RackID: "rack-delhi-a-07", CPUSockets: 2, CPUCores: 48, MemoryGiB: 768},
		{Name: "esx-delhi-a-17.airtelcloud.local", ID: "host-1017", Cluster: clusterByName[clusterDelhiA],
			RackID: "rack-delhi-a-07", CPUSockets: 2, CPUCores: 48, MemoryGiB: 768, IncidentRole: roleDegradedHost},
		{Name: "esx-delhi-a-22.airtelcloud.local", ID: "host-1022", Cluster: clusterByName[clusterDelhiA],
			RackID: "rack-delhi-a-09", CPUSockets: 2, CPUCores: 64, MemoryGiB: 1024},
		{Name: "esx-delhi-b-09.airtelcloud.local", ID: "host-2009", Cluster: clusterByName[clusterDelhiB],
			RackID: "rack-delhi-b-03", CPUSockets: 2, CPUCores: 48, MemoryGiB: 768},
	}
	for _, h := range hosts {
		h.state = newHostState(h)
	}
	c.Hosts = hosts
	hostByID := map[string]*Host{}
	for _, h := range hosts {
		hostByID[h.ID] = h
	}

	datastores := []*Datastore{
		{Name: "ds-gold-delhi-01", ID: "datastore-201", Type: "vmfs",
			StorageArray: "san-delhi-gold-01", ServiceTier: "gold", CapacityGiB: 32768},
		{Name: "ds-gold-delhi-02", ID: "datastore-202", Type: "vmfs",
			StorageArray: "san-delhi-gold-01", ServiceTier: "gold", CapacityGiB: 32768,
			IncidentRole: roleDegradedDS},
		{Name: "ds-silver-delhi-01", ID: "datastore-301", Type: "vmfs",
			StorageArray: "san-delhi-silver-01", ServiceTier: "silver", CapacityGiB: 65536},
	}
	for _, d := range datastores {
		d.state = newDatastoreState(d)
	}
	c.Datastores = datastores
	dsByID := map[string]*Datastore{}
	for _, d := range datastores {
		dsByID[d.ID] = d
	}

	type vmSpec struct {
		Name, UUID, MOID, Tenant, Workload, OS, HostID, DSID string
		VCPU, MemoryGiB                                      int
		IncidentRole                                         string
	}
	vmSpecs := []vmSpec{
		{Name: "vm-bajaj-pg-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010001", MOID: "vm-7101",
			Tenant: "tenant_bajaj_finance", Workload: "postgres_primary", OS: "ubuntu-22.04",
			VCPU: 8, MemoryGiB: 32, HostID: "host-1017", DSID: "datastore-202",
			IncidentRole: rolePrimaryVM},
		{Name: "vm-bajaj-app-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010002", MOID: "vm-7102",
			Tenant: "tenant_bajaj_finance", Workload: "app_server", OS: "ubuntu-22.04",
			VCPU: 4, MemoryGiB: 16, HostID: "host-1015", DSID: "datastore-201"},
		{Name: "vm-indigo-pg-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010003", MOID: "vm-7201",
			Tenant: "tenant_indigo_ops", Workload: "postgres_primary", OS: "rhel-8.8",
			VCPU: 8, MemoryGiB: 32, HostID: "host-1017", DSID: "datastore-202",
			IncidentRole: roleAtRiskVM},
		{Name: "vm-apollo-pg-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010004", MOID: "vm-7301",
			Tenant: "tenant_apollo_health", Workload: "postgres_primary", OS: "ubuntu-22.04",
			VCPU: 4, MemoryGiB: 16, HostID: "host-2009", DSID: "datastore-301"},
		{Name: "vm-hdfc-pg-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010005", MOID: "vm-7401",
			Tenant: "tenant_hdfc_life", Workload: "postgres_primary", OS: "rhel-9.2",
			VCPU: 8, MemoryGiB: 64, HostID: "host-1022", DSID: "datastore-201"},
		// Noisy neighbor for vm_cpu_ready_contention profile — shares host-1017 with vm-bajaj-pg-01.
		{Name: "vm-acme-batch-01", UUID: "423e6a9b-74c3-4a4f-8b0a-8a3101010006", MOID: "vm-7501",
			Tenant: "tenant_acme_retail", Workload: "batch_worker", OS: "ubuntu-22.04",
			VCPU: 8, MemoryGiB: 32, HostID: "host-1017", DSID: "datastore-201",
			IncidentRole: roleNoisyNeighbor},
	}
	for _, vs := range vmSpecs {
		host := hostByID[vs.HostID]
		ds := dsByID[vs.DSID]
		tenant := tenantByID[vs.Tenant]
		vm := &VM{
			Name: vs.Name, UUID: vs.UUID, MOID: vs.MOID,
			Tenant: tenant, WorkloadRole: vs.Workload, OS: vs.OS,
			VCPU: vs.VCPU, MemoryGiB: vs.MemoryGiB,
			Host: host, Datastore: ds, Cluster: host.Cluster,
			Region: host.Cluster.Region, AZ: host.Cluster.AZ,
			IncidentRole: vs.IncidentRole,
		}
		vm.state = newVMState(vm)
		c.VMs = append(c.VMs, vm)
	}
	vmByName := map[string]*VM{}
	for _, v := range c.VMs {
		vmByName[v.Name] = v
	}

	type pgSpec struct {
		Name, Cluster, Role, Version, Database, Tenant, VMName, Tier, IncidentRole string
		Port                                                                       int
		SLOTargetMs                                                                float64
	}
	pgSpecs := []pgSpec{
		{Name: "pg-bajaj-01", Cluster: "pg-bajaj-prod", Role: "primary", Version: "14.11",
			Database: "customerdb", Port: 5432, Tenant: "tenant_bajaj_finance",
			VMName: "vm-bajaj-pg-01", Tier: "gold", SLOTargetMs: 120, IncidentRole: rolePrimaryPG},
		{Name: "pg-indigo-01", Cluster: "pg-indigo-prod", Role: "primary", Version: "14.9",
			Database: "opsdb", Port: 5432, Tenant: "tenant_indigo_ops",
			VMName: "vm-indigo-pg-01", Tier: "gold", SLOTargetMs: 120, IncidentRole: roleAtRiskPG},
		{Name: "pg-apollo-01", Cluster: "pg-apollo-prod", Role: "primary", Version: "15.6",
			Database: "healthdb", Port: 5432, Tenant: "tenant_apollo_health",
			VMName: "vm-apollo-pg-01", Tier: "silver", SLOTargetMs: 180},
		{Name: "pg-hdfc-01", Cluster: "pg-hdfc-prod", Role: "primary", Version: "15.5",
			Database: "policydw", Port: 5432, Tenant: "tenant_hdfc_life",
			VMName: "vm-hdfc-pg-01", Tier: "gold", SLOTargetMs: 120},
	}
	for _, ps := range pgSpecs {
		pg := &PGInstance{
			Name: ps.Name, Cluster: ps.Cluster, Role: ps.Role, Version: ps.Version,
			Database: ps.Database, Port: ps.Port,
			Tenant: tenantByID[ps.Tenant], VM: vmByName[ps.VMName],
			ServiceTier: ps.Tier, SLOName: "postgres_p95_latency", SLOTargetMs: ps.SLOTargetMs,
			IncidentRole: ps.IncidentRole,
		}
		pg.state = newPGState(pg)
		c.PGInstances = append(c.PGInstances, pg)
	}

	return c
}

// VMsOnHost returns every VM scheduled on the given host. Used by the
// vmware_host_vm_count gauge and the host_contention log.
func (c *Catalog) VMsOnHost(hostID string) []*VM {
	var out []*VM
	for _, v := range c.VMs {
		if v.Host != nil && v.Host.ID == hostID {
			out = append(out, v)
		}
	}
	return out
}

// VMsOnDatastore returns every VM whose primary data lives on the datastore.
// Used for blast-radius queries.
func (c *Catalog) VMsOnDatastore(dsID string) []*VM {
	var out []*VM
	for _, v := range c.VMs {
		if v.Datastore != nil && v.Datastore.ID == dsID {
			out = append(out, v)
		}
	}
	return out
}

// PGForVM returns the PostgreSQL instance hosted on the given VM, or nil.
func (c *Catalog) PGForVM(vmName string) *PGInstance {
	for _, pg := range c.PGInstances {
		if pg.VM != nil && pg.VM.Name == vmName {
			return pg
		}
	}
	return nil
}
