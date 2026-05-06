// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package controlplane

import "github.com/cardinalhq/griffin-commerce-demo/common/faults"

// KnobCatalog is the authoritative list of supported knobs. The UI renders
// param inputs from this list, and PUT /admin/faults validates incoming
// knob keys against it.
//
// Adding a new knob is server-side only: append an entry here and add the
// hook site in the relevant service. The frontend picks it up automatically
// from GET /admin/faults/catalog.
var KnobCatalog = []faults.KnobDefinition{
	{
		Key:         "catalog.error",
		Service:     faults.ServiceCatalog,
		Kind:        faults.KindError,
		Description: "Return an error for GET /api/products/{id} when the id matches Target. Cascades into cart (failed add-to-cart) and recommendations (failed cache refresh).",
		Params: []faults.ParamSpec{
			{Name: "target", Type: "string", Required: true, Default: "PROD-001", Description: "Product ID to fail (e.g. PROD-001)"},
			{Name: "statusCode", Type: "int", Min: 400, Max: 599, Default: 500, Description: "HTTP status code returned"},
		},
		Guidance: "Pick a popular product. detect_outliers on griffin.catalog.product.requests_total with focus_tags=[\"product_id\",\"http_status_code\"] should flag the targeted product.",
	},
	{
		Key:         "catalog.slow",
		Service:     faults.ServiceCatalog,
		Kind:        faults.KindSlow,
		Description: "Add latency to every catalog response. Cart's outbound calls inherit this latency (5s timeout caps it).",
		Params: []faults.ParamSpec{
			{Name: "latencyMs", Type: "int", Min: 100, Max: 4000, Default: 2000, Description: "Additional latency per request, in milliseconds"},
		},
		Guidance: "Keep latencyMs ≤ 4000 to avoid converting slowness into cart-side timeout errors. detect_anomalies on http.server.duration with aggregation=p99.",
	},
	{
		Key:         "cart.error",
		Service:     faults.ServiceCart,
		Kind:        faults.KindError,
		Description: "Return errors from cart operations with the given probability.",
		Params: []faults.ParamSpec{
			{Name: "probability", Type: "float", Min: 0, Max: 1, Default: 0.5, Description: "Per-request probability of failure"},
			{Name: "statusCode", Type: "int", Min: 400, Max: 599, Default: 500, Description: "HTTP status code returned"},
		},
	},
	{
		Key:         "cart.outlier",
		Service:     faults.ServiceCart,
		Kind:        faults.KindOutlier,
		Description: "Add a long latency on a small fraction of cart requests. Demonstrates p99 spikes without affecting p50/mean.",
		Params: []faults.ParamSpec{
			{Name: "probability", Type: "float", Min: 0.001, Max: 0.2, Default: 0.05, Description: "Fraction of requests delayed"},
			{Name: "latencyMs", Type: "int", Min: 5000, Max: 60000, Default: 30000, Description: "Outlier delay in ms"},
		},
		Guidance: "detect_anomalies on griffin.cart.operation.duration_ms aggregation=p99 — p99 jumps while p50 holds.",
	},
	{
		Key:         "payment.fail",
		Service:     faults.ServicePayment,
		Kind:        faults.KindError,
		Description: "Override the payment processor's failure_rate to Probability for the named processor (Target). Without Target, applies to whichever processor each request happens to land on (no clean cohort signal).",
		Params: []faults.ParamSpec{
			{Name: "target", Type: "string", Default: "kittycard", Description: "Processor name to fail: puppypay | kittycard | doggiecoin (empty = all)"},
			{Name: "probability", Type: "float", Min: 0, Max: 1, Default: 0.8, Description: "Effective failure rate"},
		},
		Guidance: "Target=kittycard (or another processor) gives detect_outliers a clean per-processor cohort.",
	},
	{
		Key:         "payment.gc-storm",
		Service:     faults.ServicePayment,
		Kind:        faults.KindGCStorm,
		Description: "Generate ~100MB/s heap churn AND call runtime.GC() periodically to produce real STW pauses. runtime.GC() alone on a small heap is a no-op — the churn is what makes pauses visible.",
		Params: []faults.ParamSpec{
			{Name: "latencyMs", Type: "int", Min: 50, Max: 2000, Default: 200, Description: "Interval between runtime.GC() calls in ms"},
		},
	},
	{
		Key:         "shipping.fail",
		Service:     faults.ServiceShipping,
		Kind:        faults.KindError,
		Description: "Override a shipping carrier's failure_rate to Probability for the named carrier (Target).",
		Params: []faults.ParamSpec{
			{Name: "target", Type: "string", Required: true, Default: "catcarrier", Description: "Carrier ID: ponyexpress | avianair | catcarrier"},
			{Name: "probability", Type: "float", Min: 0, Max: 1, Default: 0.8, Description: "Effective failure rate"},
		},
	},
	{
		Key:         "images.slow",
		Service:     faults.ServiceImages,
		Kind:        faults.KindSlow,
		Description: "Add latency to every image fetch (both /static/* and /api/images/product/{id}). Sets Cache-Control: no-store while active so browser reloads always hit the slow path.",
		Params: []faults.ParamSpec{
			{Name: "latencyMs", Type: "int", Min: 200, Max: 5000, Default: 1500, Description: "Per-image delay in ms"},
		},
	},
	{
		Key:         "recs.memleak",
		Service:     faults.ServiceRecommendations,
		Kind:        faults.KindMemleak,
		Description: "Spawn a goroutine that appends 1MB padded snapshots to a never-freed slice every 100ms (~10MB/s). RSS climbs visibly; GC pressure rises with the live set.",
		Params:      []faults.ParamSpec{},
	},
	{
		Key:         "global.cpu-burn-traffic",
		Service:     faults.ServiceGlobal,
		Kind:        faults.KindCPUBurn,
		Description: "Per-request CPU spin loop in middleware. Latency cascades through all services. CPU saturation requires traffic.",
		Params: []faults.ParamSpec{
			{Name: "latencyMs", Type: "int", Min: 5, Max: 500, Default: 50, Description: "Per-request burn duration in ms"},
		},
	},
	{
		Key:         "global.cpu-burn-bg",
		Service:     faults.ServiceGlobal,
		Kind:        faults.KindCPUBurn,
		Description: "Background CPU saturation independent of traffic. Spawns runtime.NumCPU() spin goroutines per service. latencyMs=0 means full burn; >0 means duty-cycled (spin LatencyMs ms then sleep LatencyMs ms).",
		Params: []faults.ParamSpec{
			{Name: "latencyMs", Type: "int", Min: 0, Max: 500, Default: 0, Description: "0 = full burn; >0 = sleep ms between spin bursts"},
		},
	},
	{
		Key:         "cart.poison-product",
		Service:     faults.ServiceCart,
		Kind:        faults.KindError,
		Description: "Returns a deliberately uninformative 500 from any cart operation whose cart contains Target. The actual cause is logged with trace_id, demonstrating the side-drawer logs UX where logs explain failures the trace alone doesn't.",
		Params: []faults.ParamSpec{
			{Name: "target", Type: "string", Required: true, Default: "PROD-003", Description: "Product ID that, when present in a cart, poisons every operation on it"},
		},
		Guidance: "Add Target to a cart, then watch a cart op fail with a generic error in the response. Open the trace, click the failed span, switch to logs tab — the structured 'cart contains tainted item' log reveals the cause.",
	},
	{
		Key:         "loadgen.flood",
		Service:     faults.ServiceLoadgen,
		Kind:        faults.KindFlood,
		Description: "Locust ramps users from baseline to a flood target. Phase 4.",
		Params: []faults.ParamSpec{
			{Name: "probability", Type: "float", Min: 0, Max: 1, Default: 1, Description: "Target user count fraction (0–1 maps to 0–500 users)"},
			{Name: "latencyMs", Type: "int", Min: 1000, Max: 120000, Default: 30000, Description: "Ramp duration in ms"},
		},
	},
}

func lookupDefinition(key string) (faults.KnobDefinition, bool) {
	for _, def := range KnobCatalog {
		if def.Key == key {
			return def, true
		}
	}
	return faults.KnobDefinition{}, false
}
