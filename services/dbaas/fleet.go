// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

// CustomerProfile captures how one tenant's workload feels different from
// the others on a DBaaS dashboard. Per-instance jitter alone makes every
// customer look identical once aggregated, so each customer gets a profile
// that's applied as a scale factor / offset on the per-instance baselines.
//
// Profile fields fan out across two kinds of signals:
//   - Volume: QPSScale, WriteFraction → affect totals (queries, IOPS, conns)
//   - Quality: BaselineLatencyMult, BaselineErrorRate, AvailabilityFloor,
//     BufferHitFloor → affect SLI tiles (latency, success rate, availability,
//     cache hit ratio) so each customer's SLO Snapshot looks distinct
//     in steady state.
type CustomerProfile struct {
	// QPSScale multiplies the per-instance baseline QPS (120 qps). Driven
	// by customer business profile: banks are heavy traffic, IT services
	// smaller, telco/retail consumer-scale.
	QPSScale float64
	// WriteFraction is the share of queries that are writes (insert/update/
	// delete/ddl). Banks lean read-heavy; ERP and consumer apps are mixed.
	WriteFraction float64
	// BaselineLatencyMult scales the (p50, p95, p99) latency tuple per op.
	// Banks have tight query plans (mult ~0.7); ERP / analytics have larger
	// scans (mult ~2.5). Reads as p99 query latency on the SLO Snapshot tile.
	BaselineLatencyMult float64
	// BaselineErrorRate is the per-query error probability in steady state
	// (fan out across the three baseline SQLSTATEs). Banks: ~0.0001;
	// commodity tiers: ~0.001. Drives the Query Success Rate SLO tile.
	BaselineErrorRate float64
	// AvailabilityFloor is the steady-state db_up value. 1.0 for premium
	// tiers; slightly below 1.0 for business tier where the simulator
	// occasionally flips db_up=0 for some instances. Drives the
	// Availability SLO tile.
	AvailabilityFloor float64
	// BufferHitFloor is the customer's typical buffer cache hit ratio
	// floor. Banks cache hot data (~0.99); analytical / ERP workloads
	// scan cold tables (~0.94).
	BufferHitFloor float64
	// StorageTier is the *dominant* tier across the customer's volumes.
	// 80% of the customer's instances land on this tier; 20% drift to an
	// adjacent tier so the storage-class breakdown isn't a single bar.
	StorageTier string // "io2-50k" | "io2-25k" | "gp3"
}

// CustomerConfig is one tenant in the Airtel DBaaS demo fleet.
type CustomerConfig struct {
	ID      string // resource-attribute customer.id
	Name    string // human-friendly name
	Tier    string // "enterprise" | "business"
	DBs     int    // number of synthetic DB instances
	Profile CustomerProfile
}

// Fleet matches §2.3 of docs/plans/airtel-demo-plan.md in the conductor repo.
// HDFC is the scenario victim + Griffin owner; the rest are background
// tenants whose DBs show up in the customer-name dropdown.
//
// Profile values give each customer a distinct workload shape so dashboards
// visibly differ as the operator switches between tenants:
//   - HDFC Bank / Kotak Bank — banking, read-heavy, high cache hit, premium tier
//   - Tata Motors — manufacturing ERP, write-heavy, lower cache hit, standard tier
//   - Reliance Jio — consumer-scale telco/retail, huge volume, mixed workload
//   - IOCL — energy / supply chain, moderate steady, commodity tier
//   - Infosys — IT services, small footprint, business tier on gp3
var Fleet = []CustomerConfig{
	{
		ID: "c-hdfc-bank", Name: "HDFC Bank", Tier: "enterprise", DBs: 32,
		Profile: CustomerProfile{
			QPSScale: 1.2, WriteFraction: 0.18,
			BaselineLatencyMult: 0.7, BaselineErrorRate: 0.0001, AvailabilityFloor: 1.0,
			BufferHitFloor: 0.99, StorageTier: "io2-50k",
		},
	},
	{
		ID: "c-kotak-bank", Name: "Kotak Mahindra Bank", Tier: "enterprise", DBs: 38,
		Profile: CustomerProfile{
			QPSScale: 1.4, WriteFraction: 0.22,
			BaselineLatencyMult: 0.8, BaselineErrorRate: 0.0002, AvailabilityFloor: 1.0,
			BufferHitFloor: 0.99, StorageTier: "io2-50k",
		},
	},
	{
		ID: "c-tata-motors", Name: "Tata Motors", Tier: "enterprise", DBs: 14,
		Profile: CustomerProfile{
			QPSScale: 0.8, WriteFraction: 0.45,
			BaselineLatencyMult: 2.5, BaselineErrorRate: 0.0008, AvailabilityFloor: 1.0,
			BufferHitFloor: 0.94, StorageTier: "io2-25k",
		},
	},
	{
		ID: "c-reliance-jio", Name: "Reliance Jio", Tier: "enterprise", DBs: 26,
		Profile: CustomerProfile{
			QPSScale: 2.0, WriteFraction: 0.32,
			BaselineLatencyMult: 1.2, BaselineErrorRate: 0.0005, AvailabilityFloor: 1.0,
			BufferHitFloor: 0.97, StorageTier: "io2-25k",
		},
	},
	{
		ID: "c-iocl", Name: "Indian Oil Corporation", Tier: "enterprise", DBs: 18,
		Profile: CustomerProfile{
			QPSScale: 0.6, WriteFraction: 0.28,
			BaselineLatencyMult: 1.4, BaselineErrorRate: 0.0004, AvailabilityFloor: 1.0,
			BufferHitFloor: 0.96, StorageTier: "gp3",
		},
	},
	{
		ID: "c-infosys", Name: "Infosys", Tier: "business", DBs: 12,
		Profile: CustomerProfile{
			QPSScale: 0.4, WriteFraction: 0.35,
			BaselineLatencyMult: 1.9, BaselineErrorRate: 0.0012, AvailabilityFloor: 0.998,
			BufferHitFloor: 0.92, StorageTier: "gp3",
		},
	},
}

// DBInstance is one simulated managed-Postgres instance. Deterministic
// IDs let scenarios target a specific instance by name (e.g.
// hdfc-prod-03 is the volume-near-full victim). InstanceState in state.go
// owns the instance-construction loop now; this struct is just the value
// carrier the metric callback reads from.
type DBInstance struct {
	CustomerID string
	DBID       string
	Tier       string
	Role       string // "primary" | "replica"
	PgVersion  string
	Up         bool
}

// customerPrefix turns "c-hdfc-bank" into "hdfc". One-token prefix is plenty
// for demo-readability and keeps the db_id labels short on the dashboard.
func customerPrefix(customerID string) string {
	// strip the "c-" prefix, then take the first hyphen-delimited token
	s := customerID
	if len(s) > 2 && s[:2] == "c-" {
		s = s[2:]
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			return s[:i]
		}
	}
	return s
}
