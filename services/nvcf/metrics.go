// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// metrics.go registers the M1 subset of Table A. One async callback walks
// the catalog at the SDK's collection cadence and emits each instrument
// for the right cohort.
//
// All names are verbatim per docs/specs/nvcf.md §"Table A". Anything
// invented for the synth (none in M1) would be namespaced under
// nvcf_demo.* per Table B.

const instrumentScope = "github.com/cardinalhq/griffin-commerce-demo/services/nvcf"

// startTime fixes the process boot, used to grow Counter values
// monotonically as `rate * elapsed`.
var startTime = time.Now()

type instruments struct {
	// invocation plane
	functionRequestTotal   metric.Float64ObservableCounter
	functionRequestLatency metric.Float64ObservableGauge

	// stargate sidecar (function-workload-shape)
	stargateTTFTSeconds      metric.Float64ObservableGauge
	stargateOutputTPS        metric.Float64ObservableGauge
	stargateKVCacheUsed      metric.Float64ObservableGauge
	stargateKVCacheCapacity  metric.Float64ObservableGauge
	stargateRequestsInflight metric.Float64ObservableGauge

	// state-metrics service
	functionQueueDepth metric.Float64ObservableGauge

	// autoscaler (control plane)
	autoscalerCurrent metric.Float64ObservableGauge
	autoscalerDesired metric.Float64ObservableGauge

	// NVCA (compute plane)
	nvcaContainerCrashTotal metric.Float64ObservableCounter

	// DCGM (GPU)
	dcgmGPUUtil metric.Float64ObservableGauge
}

// RegisterMetrics builds the instruments and registers the synth callback.
func RegisterMetrics(ctx context.Context, catalog *Catalog, scenario *Scenario) error {
	meter := otel.Meter(instrumentScope)
	ins := &instruments{}
	if err := registerAll(meter, ins); err != nil {
		return err
	}
	obs := allObservables(ins)
	_, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			now := time.Now()
			for _, srv := range catalog.InferenceServers {
				observeStargate(o, ins, catalog, srv, scenario, now)
			}
			for _, v := range catalog.Versions {
				observeAutoscaler(o, ins, catalog, v, scenario, now)
				for _, acct := range catalog.Accounts {
					observeFunctionRequest(o, ins, catalog, v, acct, scenario, now)
					observeFunctionQueueDepth(o, ins, catalog, v, acct, scenario, now)
				}
			}
			for _, inst := range catalog.Instances {
				observeDCGM(o, ins, catalog, inst, scenario, now)
			}
			for _, cl := range catalog.Clusters {
				observeNVCAContainerCrash(o, ins, cl, scenario, now)
			}
			return nil
		},
		obs...,
	)
	if err != nil {
		return fmt.Errorf("register nvcf callback: %w", err)
	}
	slog.InfoContext(ctx, "NVCF synth metrics registered",
		"functions", len(catalog.Functions),
		"versions", len(catalog.Versions),
		"accounts", len(catalog.Accounts),
		"clusters", len(catalog.Clusters),
		"instances", len(catalog.Instances),
		"inference_servers", len(catalog.InferenceServers),
	)
	return nil
}

func registerAll(meter metric.Meter, ins *instruments) error {
	var err error
	ins.functionRequestTotal, err = meter.Float64ObservableCounter(MetricFunctionRequestTotal,
		metric.WithDescription("Cumulative completed requests for a function (gRPC proxy)"),
		metric.WithUnit("1"))
	if err != nil {
		return err
	}
	ins.functionRequestLatency, err = meter.Float64ObservableGauge(MetricFunctionRequestLatency,
		metric.WithDescription("Function request latency p95 (state-metrics-service histogram surrogate, seconds)"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}
	ins.stargateTTFTSeconds, err = meter.Float64ObservableGauge(MetricStargateTTFTSeconds,
		metric.WithDescription("Time to first token, per inference server (stargate-client)"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}
	ins.stargateOutputTPS, err = meter.Float64ObservableGauge(MetricStargateOutputTPS,
		metric.WithDescription("Output tokens per second, per model (stargate-client)"))
	if err != nil {
		return err
	}
	ins.stargateKVCacheUsed, err = meter.Float64ObservableGauge(MetricStargateKVCacheUsed,
		metric.WithDescription("KV cache used tokens (stargate-client)"))
	if err != nil {
		return err
	}
	ins.stargateKVCacheCapacity, err = meter.Float64ObservableGauge(MetricStargateKVCacheCapacity,
		metric.WithDescription("KV cache capacity tokens (stargate-client)"))
	if err != nil {
		return err
	}
	ins.stargateRequestsInflight, err = meter.Float64ObservableGauge(MetricStargateRequestsInflight,
		metric.WithDescription("Inflight requests, per model (stargate-client)"))
	if err != nil {
		return err
	}
	ins.functionQueueDepth, err = meter.Float64ObservableGauge(MetricFunctionQueueDepth,
		metric.WithDescription("Function queue depth, per account_name + function (state-metrics-service)"))
	if err != nil {
		return err
	}
	ins.autoscalerCurrent, err = meter.Float64ObservableGauge(MetricAutoscalerCurrentInstances,
		metric.WithDescription("Current instance count for a function (autoscaler)"))
	if err != nil {
		return err
	}
	ins.autoscalerDesired, err = meter.Float64ObservableGauge(MetricAutoscalerDesiredInstances,
		metric.WithDescription("Desired instance count for a function (autoscaler)"))
	if err != nil {
		return err
	}
	ins.nvcaContainerCrashTotal, err = meter.Float64ObservableCounter(MetricNVCAContainerCrashTotal,
		metric.WithDescription("Cumulative container crashes per cluster"),
		metric.WithUnit("1"))
	if err != nil {
		return err
	}
	ins.dcgmGPUUtil, err = meter.Float64ObservableGauge(MetricDCGMGPUUtil,
		metric.WithDescription("GPU utilization percent (DCGM allowlist)"),
		metric.WithUnit("%"))
	if err != nil {
		return err
	}
	return nil
}

func allObservables(ins *instruments) []metric.Observable {
	return []metric.Observable{
		ins.functionRequestTotal, ins.functionRequestLatency,
		ins.stargateTTFTSeconds, ins.stargateOutputTPS,
		ins.stargateKVCacheUsed, ins.stargateKVCacheCapacity,
		ins.stargateRequestsInflight,
		ins.functionQueueDepth,
		ins.autoscalerCurrent, ins.autoscalerDesired,
		ins.nvcaContainerCrashTotal,
		ins.dcgmGPUUtil,
	}
}

// -- per-entity observers --

func observeStargate(o metric.Observer, ins *instruments, cat *Catalog, srv *InferenceServer, sc *Scenario, now time.Time) {
	fn := cat.FunctionByID(srv.FunctionID)
	if fn == nil {
		return
	}

	commonAttrs := []attribute.KeyValue{
		attribute.String("model", srv.Model),
		attribute.String("function_id", srv.FunctionID),
		attribute.String("function_version_id", srv.FunctionVersionID),
		attribute.String("inference_server_id", srv.InferenceServerID),
		attribute.String("nvca_cluster_name", srv.NVCAClusterName),
		attribute.String("device", srv.Device),
		attribute.String("routing_key", srv.Model+":default"),
	}

	if fn.BaseTTFTSec > 0 {
		// TTFT baseline ±15% jitter. Scenario can push this up for the
		// ttft-regression knob on the targeted function_version_id.
		baseline := Range{Lo: fn.BaseTTFTSec * 0.85, Hi: fn.BaseTTFTSec * 1.15}
		seed := hash("ttft", srv.InferenceServerID)
		v := sc.RampedValue(selectorFunctionVersion(srv.FunctionVersionID), MetricStargateTTFTSeconds, baseline, seed, now)
		o.ObserveFloat64(ins.stargateTTFTSeconds, v, metric.WithAttributes(commonAttrs...))
	}

	if fn.BaseOutputTPS > 0 {
		// Output tokens/sec, per model only (not per server — that's how
		// stargate emits it).
		modelAttrs := []attribute.KeyValue{
			attribute.String("model", srv.Model),
		}
		baseline := Range{Lo: fn.BaseOutputTPS * 0.85, Hi: fn.BaseOutputTPS * 1.15}
		seed := hash("tps", srv.Model)
		v := sc.RampedValue(selectorFunction(srv.FunctionID), MetricStargateOutputTPS, baseline, seed, now)
		o.ObserveFloat64(ins.stargateOutputTPS, v, metric.WithAttributes(modelAttrs...))

		// KV cache: capacity static, used baseline 40-60% of capacity.
		const capacity = 32768.0
		usedBaseline := Range{Lo: capacity * 0.40, Hi: capacity * 0.60}
		usedSeed := hash("kv", srv.Model)
		used := sc.RampedValue(selectorFunction(srv.FunctionID), MetricStargateKVCacheUsed, usedBaseline, usedSeed, now)
		o.ObserveFloat64(ins.stargateKVCacheUsed, used, metric.WithAttributes(modelAttrs...))
		o.ObserveFloat64(ins.stargateKVCacheCapacity, capacity, metric.WithAttributes(modelAttrs...))

		// Inflight requests: small steady count.
		inflightBaseline := Range{Lo: 2, Hi: 12}
		inflightSeed := hash("inflight", srv.Model)
		inflight := sc.RampedValue(selectorFunction(srv.FunctionID), MetricStargateRequestsInflight, inflightBaseline, inflightSeed, now)
		o.ObserveFloat64(ins.stargateRequestsInflight, inflight, metric.WithAttributes(modelAttrs...))
	}
}

func observeFunctionRequest(o metric.Observer, ins *instruments, cat *Catalog, v *FunctionVersion, acct *Account, sc *Scenario, now time.Time) {
	fn := cat.FunctionByID(v.FunctionID)
	if fn == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("function_id", v.FunctionID),
		attribute.String("function_version_id", v.FunctionVersionID),
		attribute.String("account_name", acct.AccountName),
		attribute.String("account_display_name", acct.AccountDisplayName),
	}

	// Cumulative request count = rate * elapsed. Rate varies per
	// (function, account, version) using the account's weight.
	elapsed := now.Sub(startTime).Seconds()
	baseRate := 4.0 * acct.TrafficWeight // req/s
	if v.VersionLabel == "v2" {
		baseRate *= 0.7 // v2 still ramping
	}
	cumulative := baseRate * elapsed
	o.ObserveFloat64(ins.functionRequestTotal, cumulative, metric.WithAttributes(attrs...))

	// Function request latency p95 — derive a steady value from baseline TTFT
	// plus a fixed gateway/queue tax. Scenario can bend this per
	// function_version for ttft-regression.
	latAttrs := []attribute.KeyValue{
		attribute.String("function_id", v.FunctionID),
		attribute.String("function_version_id", v.FunctionVersionID),
	}
	baseLat := fn.BaseTTFTSec + 0.04
	baseline := Range{Lo: baseLat * 0.9, Hi: baseLat * 1.1}
	seed := hash("flat", v.FunctionVersionID)
	v95 := sc.RampedValue(selectorFunctionVersion(v.FunctionVersionID), MetricFunctionRequestLatency, baseline, seed, now)
	o.ObserveFloat64(ins.functionRequestLatency, v95, metric.WithAttributes(latAttrs...))
}

func observeFunctionQueueDepth(o metric.Observer, ins *instruments, cat *Catalog, v *FunctionVersion, acct *Account, sc *Scenario, now time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("function_id", v.FunctionID),
		attribute.String("function_version_id", v.FunctionVersionID),
		attribute.String("account_name", acct.AccountName),
		attribute.String("account_display_name", acct.AccountDisplayName),
	}
	baseline := Range{Lo: 1, Hi: 8 * acct.TrafficWeight}
	seed := hash("qd", v.FunctionVersionID+":"+acct.AccountName)
	val := sc.RampedValue(selectorAccount(acct.AccountName), MetricFunctionQueueDepth, baseline, seed, now)
	o.ObserveFloat64(ins.functionQueueDepth, val, metric.WithAttributes(attrs...))
}

func observeAutoscaler(o metric.Observer, ins *instruments, cat *Catalog, v *FunctionVersion, sc *Scenario, now time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("function_id", v.FunctionID),
		attribute.String("function_version_id", v.FunctionVersionID),
	}
	// Replica count per version: how many inference servers serve this version.
	desired := float64(len(cat.ServersByVersion[v.FunctionVersionID]))
	// Current stays at desired except when knobs perturb it; baseline = desired.
	desiredBaseline := Range{Lo: desired, Hi: desired}
	currentBaseline := Range{Lo: desired, Hi: desired}
	seed := hash("as", v.FunctionVersionID)
	o.ObserveFloat64(ins.autoscalerDesired,
		sc.RampedValue(selectorFunctionVersion(v.FunctionVersionID), MetricAutoscalerDesiredInstances, desiredBaseline, seed, now),
		metric.WithAttributes(attrs...))
	o.ObserveFloat64(ins.autoscalerCurrent,
		sc.RampedValue(selectorFunctionVersion(v.FunctionVersionID), MetricAutoscalerCurrentInstances, currentBaseline, seed, now),
		metric.WithAttributes(attrs...))
}

func observeNVCAContainerCrash(o metric.Observer, ins *instruments, cl *Cluster, sc *Scenario, now time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("nvca_nca_id", "nca-"+cl.NVCAClusterName),
		attribute.String("nvca_cluster_name", cl.NVCAClusterName),
		attribute.String("nvca_cluster_group", cl.NVCAClusterGroup),
		attribute.String("nvca_version", "v0.1.0"),
		attribute.String("container", "function-worker"),
	}
	// Baseline: ~0 crashes (cumulative). Scenario can push for OOM-flap knob.
	baseline := Range{Lo: 0, Hi: 0}
	seed := hash("crash", cl.NVCAClusterName)
	val := sc.RampedValue(selectorCluster(cl.NVCAClusterName), MetricNVCAContainerCrashTotal, baseline, seed, now)
	o.ObserveFloat64(ins.nvcaContainerCrashTotal, val, metric.WithAttributes(attrs...))
}

func observeDCGM(o metric.Observer, ins *instruments, cat *Catalog, inst *Instance, sc *Scenario, now time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("device", inst.Device),
		attribute.String("modelName", inst.ModelName),
		attribute.String("pci_bus_id", inst.PCIBusID),
		attribute.String("Hostname", inst.Hostname),
		attribute.String("nvca_cluster_name", inst.NVCAClusterName),
		attribute.String("DCGM_FI_DRIVER_VERSION", "550.54.15"),
	}
	// GPU util baseline 50-85% — busy fleet. Thermal-throttle knob (M2)
	// will modulate this via selectorCluster.
	baseline := Range{Lo: 50, Hi: 85}
	seed := hash("util", inst.NVCAClusterName+":"+inst.Device)
	val := sc.RampedValue(selectorInstance(inst.NVCAClusterName, inst.Device), MetricDCGMGPUUtil, baseline, seed, now)
	o.ObserveFloat64(ins.dcgmGPUUtil, val, metric.WithAttributes(attrs...))
}

// hash is a deterministic seed-from-string for Range.Sample.
func hash(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}
