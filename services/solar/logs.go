// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// logs.go emits the spec §16–21 event types as OTLP log records. Every
// record carries the full correlation field set so join queries work
// without joins.

const defaultLogIntervalSeconds = 5

func logIntervalSeconds() int {
	if v := os.Getenv("SOLAR_LOG_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultLogIntervalSeconds
}

func StartLogEmitter(ctx context.Context, catalog *Catalog, scenario *Scenario) {
	logger := global.GetLoggerProvider().Logger(instrumentScope)
	interval := time.Duration(logIntervalSeconds()) * time.Second
	rng := rand.New(rand.NewSource(0x501a ^ time.Now().UnixNano()))
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				emitTick(ctx, logger, catalog, scenario, now, interval, rng)
			}
		}
	}()
}

func emitTick(ctx context.Context, logger log.Logger, c *Catalog, sc *Scenario, now time.Time, interval time.Duration, rng *rand.Rand) {
	intervalMin := interval.Minutes()
	for _, t := range c.Transformers {
		sel := selectorTransformer(t.ID)
		windingHot := sc.IsActiveOn(sel, MetricMVTrafoWindingTempC, now)
		oilLow := sc.IsActiveOn(sel, MetricMVTrafoOilFlowLPM, now)
		if windingHot {
			emitN(rng, 2*intervalMin, func() { logTransformerWinding(ctx, logger, t, now) })
		}
		if oilLow {
			emitN(rng, 1.5*intervalMin, func() { logTransformerOilFlow(ctx, logger, t, now) })
		}
		if windingHot && t.IncidentRole == rolePrimaryTransformer {
			emitN(rng, 0.4*intervalMin, func() { logTransformerBuchholz(ctx, logger, t, now) })
		}
	}
	for _, inv := range c.Inverters {
		sel := selectorInverter(inv.ID)
		hot := sc.IsActiveOn(sel, MetricInverterIGBTTempC, now)
		fanLow := sc.IsActiveOn(sel, MetricInverterCoolingFanRPM, now)
		mppt := sc.IsActiveOn(sel, MetricInverterMPPTVoltageV, now)
		if hot || fanLow {
			emitN(rng, 0.8*intervalMin, func() { logInverterDerate(ctx, logger, inv, now) })
		}
		if mppt {
			emitN(rng, 0.6*intervalMin, func() { logInverterMPPTChase(ctx, logger, inv, now) })
		}
		if fanLow && inv.IncidentRole == rolePrimaryInverter {
			emitN(rng, 0.3*intervalMin, func() { logInverterTrip(ctx, logger, inv, "over_temp", now) })
		}
	}
	for _, t := range c.Trackers {
		sel := selectorTracker(t.ID)
		if sc.IsActiveOn(sel, MetricTrackerAngleDeg, now) {
			emitN(rng, 1.5*intervalMin, func() { logTrackerStowFailed(ctx, logger, t, now) })
			emitN(rng, 0.6*intervalMin, func() { logTrackerFault(ctx, logger, t, now) })
		}
	}
	for _, m := range c.MetStations {
		sel := selectorMet(m.ID)
		if sc.IsActiveOn(sel, MetricMetSoilingLossPct, now) {
			emitN(rng, 0.8*intervalMin, func() { logSoilingThreshold(ctx, logger, m, now) })
		}
	}
	for _, o := range c.Offtakers {
		sel := selectorOfftaker(o.OfftakerID)
		if sc.IsActiveOn(sel, MetricPPADispatchDeviationPct, now) {
			emitN(rng, 1*intervalMin, func() { logPPABreach(ctx, logger, o, now) })
		}
	}
	if sc.ActiveProfileID() != "" {
		emitN(rng, 0.5*intervalMin, func() { logCorrelationDiscovered(ctx, logger, c, sc, now) })
	}
}

// emitN samples a Poisson count from lambda and invokes fn that many times,
// bounded to 50.
func emitN(rng *rand.Rand, lambda float64, fn func()) {
	n := poisson(rng, lambda)
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		fn()
	}
}

func poisson(rng *rand.Rand, lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	L := math.Exp(-lambda)
	k := 0
	p := 1.0
	for {
		k++
		p *= rng.Float64()
		if p <= L {
			return k - 1
		}
	}
}

// -- record builders --

func emit(ctx context.Context, logger log.Logger, sev log.Severity, sevText, body string, kv ...log.KeyValue) {
	r := log.Record{}
	r.SetTimestamp(time.Now())
	r.SetObservedTimestamp(time.Now())
	r.SetSeverity(sev)
	r.SetSeverityText(sevText)
	r.SetBody(log.StringValue(body))
	r.AddAttributes(kv...)
	logger.Emit(ctx, r)
}

func siteKV() []log.KeyValue {
	return []log.KeyValue{
		log.String("site_id", siteID),
		log.String("plant_name", plantName),
		log.String("region", region),
		log.String("scenario_id", "adani_khavda_solar_park"),
	}
}

func logTransformerWinding(ctx context.Context, logger log.Logger, t *MVTransformer, now time.Time) {
	winding := 96 + 12*math.Sin(float64(now.Unix()%60)/9.5) // synthetic readout
	body := fmt.Sprintf("Transformer %s winding HV %.1f°C exceeds warning threshold 90°C", t.ID, winding)
	kv := append(siteKV(),
		log.String("event_type", "mv_transformer_winding_alarm"),
		log.String("mv_transformer_id", t.ID),
		log.Float64("winding_temp_c", winding),
		log.String("severity_text", "HIGH"),
	)
	if t.Compound != nil {
		kv = append(kv, log.String("mv_compound_id", t.Compound.ID))
	}
	emit(ctx, logger, log.SeverityWarn3, "HIGH", body, kv...)
}

func logTransformerOilFlow(ctx context.Context, logger log.Logger, t *MVTransformer, now time.Time) {
	flow := 22 + 6*math.Cos(float64(now.Unix()%60)/8.0)
	body := fmt.Sprintf("Transformer %s cooling oil flow %.1f LPM below threshold 50 LPM", t.ID, flow)
	kv := append(siteKV(),
		log.String("event_type", "mv_transformer_oil_low_flow"),
		log.String("mv_transformer_id", t.ID),
		log.Float64("oil_flow_lpm", flow),
		log.String("severity_text", "WARN"),
	)
	if t.Compound != nil {
		kv = append(kv, log.String("mv_compound_id", t.Compound.ID))
	}
	emit(ctx, logger, log.SeverityWarn, "WARN", body, kv...)
}

func logTransformerBuchholz(ctx context.Context, logger log.Logger, t *MVTransformer, now time.Time) {
	body := fmt.Sprintf("Transformer %s Buchholz relay gas accumulation alarm — trend rate %.2f mL/min", t.ID, 0.6+0.1*math.Sin(float64(now.Unix()%30)))
	kv := append(siteKV(),
		log.String("event_type", "mv_transformer_buchholz"),
		log.String("mv_transformer_id", t.ID),
		log.String("severity_text", "CRITICAL"),
	)
	if t.Compound != nil {
		kv = append(kv, log.String("mv_compound_id", t.Compound.ID))
	}
	emit(ctx, logger, log.SeverityFatal, "CRITICAL", body, kv...)
}

func logInverterDerate(ctx context.Context, logger log.Logger, inv *Inverter, now time.Time) {
	temp := 88 + 8*math.Sin(float64(now.Unix()%60)/10.0)
	body := fmt.Sprintf("Inverter %s entered derate due to IGBT temp %.1f°C", inv.ID, temp)
	kv := append(siteKV(), inverterCorrelationKV(inv)...)
	kv = append(kv,
		log.String("event_type", "inverter_derate"),
		log.Float64("igbt_temp_c", temp),
		log.String("severity_text", "WARN"),
	)
	emit(ctx, logger, log.SeverityWarn, "WARN", body, kv...)
}

func logInverterTrip(ctx context.Context, logger log.Logger, inv *Inverter, reason string, now time.Time) {
	body := fmt.Sprintf("Inverter %s tripped: %s", inv.ID, reason)
	kv := append(siteKV(), inverterCorrelationKV(inv)...)
	kv = append(kv,
		log.String("event_type", "inverter_trip"),
		log.String("trip_reason", reason),
		log.String("severity_text", "ERROR"),
	)
	_ = now
	emit(ctx, logger, log.SeverityError, "ERROR", body, kv...)
}

func logInverterMPPTChase(ctx context.Context, logger log.Logger, inv *Inverter, now time.Time) {
	v := 820 + 30*math.Sin(float64(now.Unix()%30)/4.5)
	body := fmt.Sprintf("Inverter %s MPPT search oscillating, dc_v=%.0f", inv.ID, v)
	kv := append(siteKV(), inverterCorrelationKV(inv)...)
	kv = append(kv,
		log.String("event_type", "inverter_mppt_chase"),
		log.Float64("mppt_voltage_v", v),
		log.String("severity_text", "INFO"),
	)
	emit(ctx, logger, log.SeverityInfo, "INFO", body, kv...)
}

func inverterCorrelationKV(inv *Inverter) []log.KeyValue {
	kv := []log.KeyValue{
		log.String("inverter_id", inv.ID),
		log.String("inverter_vendor", inv.Vendor),
		log.String("inverter_model", inv.Model),
	}
	if inv.Station != nil {
		kv = append(kv, log.String("inverter_station_id", inv.Station.ID))
		if inv.Station.Block != nil {
			b := inv.Station.Block
			kv = append(kv, log.String("block_id", b.ID))
			if b.Offtaker != nil {
				kv = append(kv,
					log.String("offtaker_id", b.Offtaker.OfftakerID),
					log.String("ppa_id", b.Offtaker.PPAID),
				)
			}
			if b.Transformer != nil {
				kv = append(kv, log.String("mv_transformer_id", b.Transformer.ID))
				if b.Transformer.Compound != nil {
					kv = append(kv, log.String("mv_compound_id", b.Transformer.Compound.ID))
				}
			}
		}
	}
	return kv
}

func logTrackerStowFailed(ctx context.Context, logger log.Logger, t *Tracker, now time.Time) {
	cur := 12 + 4*math.Sin(float64(now.Unix()%30)/4.7)
	target := 45.0
	body := fmt.Sprintf("Tracker %s stow command failed, current %.1f° vs target %.1f°", t.ID, cur, target)
	kv := append(siteKV(),
		log.String("event_type", "tracker_stow_failed"),
		log.String("tracker_id", t.ID),
		log.Float64("current_angle_deg", cur),
		log.Float64("target_angle_deg", target),
		log.String("severity_text", "WARN"),
	)
	if t.Block != nil {
		kv = append(kv, log.String("block_id", t.Block.ID))
	}
	emit(ctx, logger, log.SeverityWarn, "WARN", body, kv...)
}

func logTrackerFault(ctx context.Context, logger log.Logger, t *Tracker, now time.Time) {
	body := fmt.Sprintf("Tracker %s motor over-current, fault_code=MOC-04", t.ID)
	kv := append(siteKV(),
		log.String("event_type", "tracker_fault"),
		log.String("tracker_id", t.ID),
		log.String("fault_code", "MOC-04"),
		log.String("severity_text", "ERROR"),
	)
	if t.Block != nil {
		kv = append(kv, log.String("block_id", t.Block.ID))
	}
	_ = now
	emit(ctx, logger, log.SeverityError, "ERROR", body, kv...)
}

func logSoilingThreshold(ctx context.Context, logger log.Logger, m *MetStation, now time.Time) {
	soil := 5.5 + 2.0*math.Sin(float64(now.Unix()%60)/9.3)
	body := fmt.Sprintf("Met %s soiling loss crossed %.1f%% (threshold 4.0%%)", m.ID, soil)
	kv := append(siteKV(),
		log.String("event_type", "met_soiling_threshold"),
		log.String("met_station_id", m.ID),
		log.Float64("soiling_loss_pct", soil),
		log.String("severity_text", "WARN"),
	)
	if m.Block != nil {
		kv = append(kv, log.String("block_id", m.Block.ID))
	}
	emit(ctx, logger, log.SeverityWarn, "WARN", body, kv...)
}

func logPPABreach(ctx context.Context, logger log.Logger, o *Offtaker, now time.Time) {
	scheduled := o.ScheduledDispatchMW
	actual := scheduled * (0.82 + 0.05*math.Sin(float64(now.Unix()%60)/11.5))
	dev := (scheduled - actual) / scheduled * 100
	body := fmt.Sprintf("PPA %s actual %.1f MW vs scheduled %.1f MW (deviation %.1f%%)",
		o.OfftakerID, actual, scheduled, dev)
	kv := append(siteKV(),
		log.String("event_type", "ppa_schedule_breach"),
		log.String("offtaker_id", o.OfftakerID),
		log.String("ppa_id", o.PPAID),
		log.Float64("actual_dispatch_mw", actual),
		log.Float64("scheduled_dispatch_mw", scheduled),
		log.Float64("dispatch_deviation_pct", dev),
		log.String("severity_text", "HIGH"),
	)
	emit(ctx, logger, log.SeverityWarn3, "HIGH", body, kv...)
}

func logCorrelationDiscovered(ctx context.Context, logger log.Logger, c *Catalog, sc *Scenario, now time.Time) {
	id := sc.ActiveProfileID()
	var body string
	kv := append(siteKV(),
		log.String("event_type", "correlation_discovered"),
		log.String("active_profile_id", id),
		log.String("severity_text", "INFO"),
	)
	switch id {
	case ProfileMVTransformerWindingOverheat:
		body = fmt.Sprintf("Correlation: transformer %s winding temp climbing → blocks %v derate → offtaker %s dispatch deviation; sibling %s in compound %s at-risk",
			mvTrafoT04A, blockIDs(c.BlocksOnTransformer(mvTrafoT04A)), offtakerSECI, mvTrafoT04B, mvCompound04)
		kv = append(kv,
			log.String("primary_mv_transformer_id", mvTrafoT04A),
			log.String("at_risk_mv_transformer_id", mvTrafoT04B),
			log.String("mv_compound_id", mvCompound04),
			log.String("primary_offtaker_id", offtakerSECI),
			log.String("at_risk_offtaker_id", offtakerGUVNL),
		)
	case ProfileInverterCoolingFault:
		body = "Correlation: inverter INV-08-02-03 cooling fan RPM=0 → IGBT temp climb → derate → station IS-08-02 AC power dip"
		kv = append(kv,
			log.String("primary_inverter_id", "INV-08-02-03"),
			log.String("inverter_station_id", "IS-08-02"),
		)
	case ProfileTrackerStowMisalignment:
		body = "Correlation: tracker TRK-12 misaligned → POA irradiance drop on MET-12-1 → block-12 PR drop → offtaker seci_phase_iii dispatch nudge"
		kv = append(kv,
			log.String("primary_tracker_id", "TRK-12"),
			log.String("block_id", "block-12"),
		)
	case ProfileStringPIDDegradation:
		body = "Correlation: inverter INV-10-03-01 string imbalance + weakest-string current drop → MPPT chase → inverter AC derate"
		kv = append(kv,
			log.String("primary_inverter_id", "INV-10-03-01"),
			log.String("inverter_station_id", "IS-10-03"),
		)
	default:
		body = "Correlation: active profile " + id
	}
	_ = now
	emit(ctx, logger, log.SeverityInfo, "INFO", body, kv...)
}

func blockIDs(bs []*Block) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.ID)
	}
	return out
}
