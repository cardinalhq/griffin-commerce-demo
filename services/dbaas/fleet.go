// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import "fmt"

// CustomerConfig is one tenant in the Airtel DBaaS demo fleet.
type CustomerConfig struct {
	ID   string // resource-attribute customer.id
	Name string // human-friendly name
	Tier string // "enterprise" | "business"
	DBs  int    // number of synthetic DB instances
}

// Fleet matches §2.3 of docs/plans/airtel-demo-plan.md in the conductor repo.
// HDFC is the scenario victim + Griffin owner; the rest are background
// tenants whose DBs show up in the customer-id dropdown.
var Fleet = []CustomerConfig{
	{ID: "c-hdfc-bank", Name: "HDFC Bank", Tier: "enterprise", DBs: 32},
	{ID: "c-kotak-bank", Name: "Kotak Mahindra Bank", Tier: "enterprise", DBs: 38},
	{ID: "c-tata-motors", Name: "Tata Motors", Tier: "enterprise", DBs: 14},
	{ID: "c-reliance-jio", Name: "Reliance Jio", Tier: "enterprise", DBs: 26},
	{ID: "c-iocl", Name: "Indian Oil Corporation", Tier: "enterprise", DBs: 18},
	{ID: "c-infosys", Name: "Infosys", Tier: "business", DBs: 12},
}

// DBInstance is one simulated managed-Postgres instance. Deterministic
// IDs let scenarios target a specific instance by name (e.g.
// hdfc-prod-03 is the volume-near-full victim).
type DBInstance struct {
	CustomerID string
	DBID       string
	Tier       string
	Role       string // "primary" | "replica"
	PgVersion  string
	Up         bool
}

// BuildInstances expands the fleet into individual DB instances. Naming
// convention: <customer-prefix>-prod-NN, where the prefix is the part of
// the customer.id after the "c-" prefix with hyphens collapsed (e.g.
// c-hdfc-bank -> hdfc-bank-prod-01). For the demo we keep it readable.
func BuildInstances() []*DBInstance {
	var out []*DBInstance
	for _, c := range Fleet {
		prefix := customerPrefix(c.ID)
		for i := 1; i <= c.DBs; i++ {
			out = append(out, &DBInstance{
				CustomerID: c.ID,
				DBID:       fmt.Sprintf("%s-prod-%02d", prefix, i),
				Tier:       c.Tier,
				Role:       "primary",
				PgVersion:  "15.6",
				Up:         true,
			})
		}
	}
	return out
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
