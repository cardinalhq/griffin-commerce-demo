// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentScope = "github.com/cardinalhq/griffin-commerce-demo/services/solar"

// Metric keys — used by scenario.go to address impact specs. Strings are
// the literal instrument names emitted on the wire.
const (
	// PPA / offtaker (analog to tenant SLO).
	MetricPPAScheduledDispatchMW  = "ppa_scheduled_dispatch_mw"
	MetricPPAActualDispatchMW     = "ppa_actual_dispatch_mw"
	MetricPPADispatchDeviationPct = "ppa_dispatch_deviation_pct"
	MetricPPAComplianceRatio      = "ppa_compliance_ratio"
	MetricPPABurnRate             = "ppa_burn_rate"
	MetricPPARevenueAtRiskINRMin  = "ppa_revenue_at_risk_inr_per_min"

	// Block.
	MetricBlockACPowerMW        = "block_ac_power_mw"
	MetricBlockPerformanceRatio = "block_performance_ratio"
	MetricBlockEnergyMWh        = "block_energy_mwh_total"
	MetricBlockAvailability     = "block_availability_ratio"

	// Inverter station (PCS).
	MetricInverterStationACPowerKW  = "inverter_station_ac_power_kw"
	MetricInverterStationACVoltageV = "inverter_station_ac_voltage_v"
	MetricInverterStationACCurrentA = "inverter_station_ac_current_a"
	MetricInverterStationGridHz     = "inverter_station_grid_frequency_hz"

	// Inverter (per unit).
	MetricInverterDCPowerKW          = "inverter_dc_power_kw"
	MetricInverterACPowerKW          = "inverter_ac_power_kw"
	MetricInverterEfficiencyPct      = "inverter_efficiency_pct"
	MetricInverterInternalTempC      = "inverter_internal_temp_c"
	MetricInverterIGBTTempC          = "inverter_igbt_temp_c"
	MetricInverterHeatsinkTempC      = "inverter_heatsink_temp_c"
	MetricInverterCoolingFanRPM      = "inverter_cooling_fan_rpm"
	MetricInverterMPPTVoltageV       = "inverter_mppt_voltage_v"
	MetricInverterMPPTCurrentA       = "inverter_mppt_current_a"
	MetricInverterDCBusVoltageV      = "inverter_dc_bus_voltage_v"
	MetricInverterDerateState        = "inverter_derate_state"
	MetricInverterEnergyKWh          = "inverter_energy_kwh_total"
	MetricInverterTHDPct             = "inverter_thd_percent"
	MetricInverterStringImbalancePct = "inverter_string_imbalance_pct"
	MetricInverterStringMinCurrentA  = "inverter_string_min_current_a"

	// MV transformer.
	MetricMVTrafoWindingTempC  = "mv_transformer_winding_temp_c"
	MetricMVTrafoOilTempC      = "mv_transformer_oil_temp_c"
	MetricMVTrafoOilLevelPct   = "mv_transformer_oil_level_pct"
	MetricMVTrafoOilFlowLPM    = "mv_transformer_cooling_oil_flow_lpm"
	MetricMVTrafoRadiatorTempC = "mv_transformer_radiator_temp_c"
	MetricMVTrafoLoadKVA       = "mv_transformer_load_kva"
	MetricMVTrafoTapPosition   = "mv_transformer_tap_position"
	MetricMVTrafoBuchholzAlarm = "mv_transformer_buchholz_alarm"

	// MV compound (outdoor bay shared by sibling transformers).
	MetricMVCompoundAmbientTempC = "mv_compound_ambient_temp_c"

	// Tracker.
	MetricTrackerAngleDeg       = "tracker_angle_deg"
	MetricTrackerTargetAngleDeg = "tracker_target_angle_deg"
	MetricTrackerMotorCurrentA  = "tracker_motor_current_a"
	MetricTrackerFaultCount     = "tracker_fault_count_total"

	// Met station.
	MetricMetIrradiancePOAWm2 = "met_irradiance_poa_w_m2"
	MetricMetIrradianceGHIWm2 = "met_irradiance_ghi_w_m2"
	MetricMetAmbientTempC     = "met_ambient_temp_c"
	MetricMetModuleTempC      = "met_module_temp_c"
	MetricMetWindSpeedMPS     = "met_wind_speed_mps"
	MetricMetHumidityPct      = "met_humidity_pct"
	MetricMetSoilingLossPct   = "met_soiling_loss_pct"

	// Substation.
	MetricSubstationExportPowerMW   = "substation_export_power_mw"
	MetricSubstationGridFrequencyHz = "substation_grid_frequency_hz"
	MetricSubstationBusbarVoltageKV = "substation_busbar_voltage_kv"
	MetricSubstationEnergyMWh       = "substation_energy_mwh_total"

	// Alert.
	MetricCardinalAlertActive = "cardinal_alert_active"
)

type instruments struct {
	// PPA
	ppaScheduled  metric.Float64ObservableGauge
	ppaActual     metric.Float64ObservableGauge
	ppaDeviation  metric.Float64ObservableGauge
	ppaCompliance metric.Float64ObservableGauge
	ppaBurn       metric.Float64ObservableGauge
	ppaRevAtRisk  metric.Float64ObservableGauge

	// Block
	blockAC          metric.Float64ObservableGauge
	blockPR          metric.Float64ObservableGauge
	blockEnergy      metric.Float64ObservableCounter
	blockAvail       metric.Float64ObservableGauge

	// Inverter station
	isACPower   metric.Float64ObservableGauge
	isACVoltage metric.Float64ObservableGauge
	isACCurrent metric.Float64ObservableGauge
	isGridHz    metric.Float64ObservableGauge

	// Inverter
	invDC            metric.Float64ObservableGauge
	invAC            metric.Float64ObservableGauge
	invEff           metric.Float64ObservableGauge
	invInternalTemp  metric.Float64ObservableGauge
	invIGBTTemp      metric.Float64ObservableGauge
	invHeatsinkTemp  metric.Float64ObservableGauge
	invFanRPM        metric.Float64ObservableGauge
	invMPPTV         metric.Float64ObservableGauge
	invMPPTI         metric.Float64ObservableGauge
	invDCBusV        metric.Float64ObservableGauge
	invDerateState   metric.Float64ObservableGauge
	invEnergy        metric.Float64ObservableCounter
	invTHD           metric.Float64ObservableGauge
	invStringImbal   metric.Float64ObservableGauge
	invStringMinCur  metric.Float64ObservableGauge

	// MV transformer
	trafoWinding  metric.Float64ObservableGauge
	trafoOil      metric.Float64ObservableGauge
	trafoOilLevel metric.Float64ObservableGauge
	trafoOilFlow  metric.Float64ObservableGauge
	trafoRadiator metric.Float64ObservableGauge
	trafoLoad     metric.Float64ObservableGauge
	trafoTap      metric.Float64ObservableGauge
	trafoBuchholz metric.Float64ObservableGauge

	// MV compound
	compoundAmbient metric.Float64ObservableGauge

	// Tracker
	trkAngle      metric.Float64ObservableGauge
	trkTarget     metric.Float64ObservableGauge
	trkMotorCur   metric.Float64ObservableGauge
	trkFaultCnt   metric.Float64ObservableCounter

	// Met
	metPOA    metric.Float64ObservableGauge
	metGHI    metric.Float64ObservableGauge
	metAmb    metric.Float64ObservableGauge
	metMod    metric.Float64ObservableGauge
	metWind   metric.Float64ObservableGauge
	metHumid  metric.Float64ObservableGauge
	metSoil   metric.Float64ObservableGauge

	// Substation
	subExport  metric.Float64ObservableGauge
	subHz      metric.Float64ObservableGauge
	subBusKv   metric.Float64ObservableGauge
	subEnergy  metric.Float64ObservableCounter

	// Alert
	alertActive metric.Float64ObservableGauge
}

func emitPerInverter() bool {
	v := os.Getenv("SOLAR_EMIT_PER_INVERTER")
	return v == "" || v == "true" || v == "1"
}

// RegisterMetrics builds every instrument and registers a single async
// callback that walks the catalog on the SDK's collection cadence.
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
			for _, off := range catalog.Offtakers {
				observeOfftaker(o, ins, off, catalog, scenario, now)
			}
			for _, b := range catalog.Blocks {
				observeBlock(o, ins, b, catalog, scenario, now)
			}
			for _, s := range catalog.Stations {
				observeStation(o, ins, s, scenario, now)
			}
			if emitPerInverter() {
				for _, inv := range catalog.Inverters {
					observeInverter(o, ins, inv, scenario, now)
				}
			}
			for _, mt := range catalog.Transformers {
				observeTransformer(o, ins, mt, scenario, now)
			}
			for _, mc := range catalog.MVCompounds {
				observeCompound(o, ins, mc, scenario, now)
			}
			for _, t := range catalog.Trackers {
				observeTracker(o, ins, t, scenario, now)
			}
			for _, m := range catalog.MetStations {
				observeMet(o, ins, m, scenario, now)
			}
			observeSubstation(o, ins, catalog.Substation, scenario, now)
			observeAlerts(o, ins, scenario, now)
			return nil
		},
		obs...,
	)
	if err != nil {
		return fmt.Errorf("register solar callback: %w", err)
	}
	slog.InfoContext(ctx, "Adani solar telemetry simulator metrics registered",
		"site", catalog.Site,
		"offtakers", len(catalog.Offtakers),
		"blocks", len(catalog.Blocks),
		"stations", len(catalog.Stations),
		"inverters", len(catalog.Inverters),
		"transformers", len(catalog.Transformers),
		"trackers", len(catalog.Trackers),
		"met_stations", len(catalog.MetStations),
		"emit_per_inverter", emitPerInverter(),
	)
	return nil
}

func allObservables(ins *instruments) []metric.Observable {
	return []metric.Observable{
		ins.ppaScheduled, ins.ppaActual, ins.ppaDeviation, ins.ppaCompliance, ins.ppaBurn, ins.ppaRevAtRisk,
		ins.blockAC, ins.blockPR, ins.blockEnergy, ins.blockAvail,
		ins.isACPower, ins.isACVoltage, ins.isACCurrent, ins.isGridHz,
		ins.invDC, ins.invAC, ins.invEff, ins.invInternalTemp, ins.invIGBTTemp,
		ins.invHeatsinkTemp, ins.invFanRPM, ins.invMPPTV, ins.invMPPTI,
		ins.invDCBusV, ins.invDerateState, ins.invEnergy, ins.invTHD,
		ins.invStringImbal, ins.invStringMinCur,
		ins.trafoWinding, ins.trafoOil, ins.trafoOilLevel, ins.trafoOilFlow,
		ins.trafoRadiator, ins.trafoLoad, ins.trafoTap, ins.trafoBuchholz,
		ins.compoundAmbient,
		ins.trkAngle, ins.trkTarget, ins.trkMotorCur, ins.trkFaultCnt,
		ins.metPOA, ins.metGHI, ins.metAmb, ins.metMod, ins.metWind, ins.metHumid, ins.metSoil,
		ins.subExport, ins.subHz, ins.subBusKv, ins.subEnergy,
		ins.alertActive,
	}
}

// -- attribute builders --

func siteAttrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("site_id", siteID),
		attribute.String("plant_name", plantName),
		attribute.String("region", region),
		attribute.String("state", stateName),
		attribute.String("country", country),
	}
}

func offtakerAttrs(o *Offtaker) []attribute.KeyValue {
	return append(siteAttrs(),
		attribute.String("offtaker_id", o.OfftakerID),
		attribute.String("ppa_id", o.PPAID),
		attribute.String("ppa_tier", o.Tier),
		attribute.String("offtaker_name", o.Name),
	)
}

func blockAttrs(b *Block) []attribute.KeyValue {
	attrs := append(siteAttrs(),
		attribute.String("block_id", b.ID),
		attribute.String("block_name", b.BlockName),
	)
	if b.Offtaker != nil {
		attrs = append(attrs,
			attribute.String("offtaker_id", b.Offtaker.OfftakerID),
			attribute.String("ppa_id", b.Offtaker.PPAID),
		)
	}
	if b.Transformer != nil {
		attrs = append(attrs,
			attribute.String("mv_transformer_id", b.Transformer.ID),
		)
		if b.Transformer.Compound != nil {
			attrs = append(attrs,
				attribute.String("mv_compound_id", b.Transformer.Compound.ID),
			)
		}
	}
	return attrs
}

func stationAttrs(s *InverterStation) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("inverter_station_id", s.ID),
	}
	attrs = append(attrs, blockAttrs(s.Block)...)
	return attrs
}

func inverterAttrs(i *Inverter) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("inverter_id", i.ID),
		attribute.String("inverter_vendor", i.Vendor),
		attribute.String("inverter_model", i.Model),
	}
	attrs = append(attrs, stationAttrs(i.Station)...)
	return attrs
}

func transformerAttrs(t *MVTransformer) []attribute.KeyValue {
	attrs := append(siteAttrs(),
		attribute.String("mv_transformer_id", t.ID),
		attribute.String("transformer_vendor", t.Vendor),
		attribute.String("transformer_model", t.Model),
		attribute.String("hv_kv", strconv.FormatFloat(t.HVKv, 'f', -1, 64)),
		attribute.String("lv_kv", strconv.FormatFloat(t.LVKv, 'f', -1, 64)),
	)
	if t.Compound != nil {
		attrs = append(attrs,
			attribute.String("mv_compound_id", t.Compound.ID),
			attribute.String("outdoor_bay", t.Compound.OutdoorBay),
		)
	}
	return attrs
}

func compoundAttrs(c *MVCompound) []attribute.KeyValue {
	return append(siteAttrs(),
		attribute.String("mv_compound_id", c.ID),
		attribute.String("outdoor_bay", c.OutdoorBay),
	)
}

func trackerAttrs(t *Tracker) []attribute.KeyValue {
	attrs := append(siteAttrs(),
		attribute.String("tracker_id", t.ID),
	)
	if t.Block != nil {
		attrs = append(attrs, attribute.String("block_id", t.Block.ID))
	}
	return attrs
}

func metAttrs(m *MetStation) []attribute.KeyValue {
	attrs := append(siteAttrs(),
		attribute.String("met_station_id", m.ID),
	)
	if m.Block != nil {
		attrs = append(attrs, attribute.String("block_id", m.Block.ID))
	}
	return attrs
}

func substationAttrs(s *Substation) []attribute.KeyValue {
	return append(siteAttrs(),
		attribute.String("substation_id", s.ID),
		attribute.String("grid_connection_point", s.GridConnectionPoint),
	)
}

// -- observers --

func observeOfftaker(o metric.Observer, ins *instruments, off *Offtaker, c *Catalog, sc *Scenario, now time.Time) {
	off.state.mu.Lock()
	defer off.state.mu.Unlock()
	sel := selectorOfftaker(off.OfftakerID)
	base := offtakerAttrs(off)

	o.ObserveFloat64(ins.ppaScheduled, off.ScheduledDispatchMW, metric.WithAttributes(base...))

	// Actual dispatch = sum of block AC power for blocks belonging to the offtaker.
	var actualMW float64
	for _, b := range c.BlocksForOfftaker(off.OfftakerID) {
		actualMW += sc.RampedValue(selectorBlock(b.ID), MetricBlockACPowerMW,
			Range{42, 48}, b.state.seed^7, now)
	}
	// Profile may override the offtaker-level rollup directly (mv_overheat scenario).
	if sc.IsActiveOn(sel, MetricPPAActualDispatchMW, now) {
		actualMW = sc.RampedValue(sel, MetricPPAActualDispatchMW, Range{actualMW * 0.95, actualMW * 1.05}, off.state.seed^1, now)
	}
	o.ObserveFloat64(ins.ppaActual, actualMW, metric.WithAttributes(base...))

	deviation := math.Abs((actualMW - off.ScheduledDispatchMW) / off.ScheduledDispatchMW * 100)
	deviation = sc.RampedValue(sel, MetricPPADispatchDeviationPct, Range{deviation, deviation}, off.state.seed^2, now)
	o.ObserveFloat64(ins.ppaDeviation, deviation, metric.WithAttributes(base...))

	compliance := sc.RampedValue(sel, MetricPPAComplianceRatio, Range{0.992, 1.0}, off.state.seed^3, now)
	o.ObserveFloat64(ins.ppaCompliance, clamp(compliance, 0, 1), metric.WithAttributes(base...))

	burn := sc.RampedValue(sel, MetricPPABurnRate, Range{0.2, 1.0}, off.state.seed^4, now)
	o.ObserveFloat64(ins.ppaBurn, burn, metric.WithAttributes(base...))

	revRisk := sc.RampedValue(sel, MetricPPARevenueAtRiskINRMin, Range{0, 250}, off.state.seed^5, now)
	o.ObserveFloat64(ins.ppaRevAtRisk, revRisk, metric.WithAttributes(base...))
}

func observeBlock(o metric.Observer, ins *instruments, b *Block, c *Catalog, sc *Scenario, now time.Time) {
	b.state.mu.Lock()
	defer b.state.mu.Unlock()
	dt := dtAdvance(&b.state.lastTick, now)
	sel := selectorBlock(b.ID)
	base := blockAttrs(b)
	ac := sc.RampedValue(sel, MetricBlockACPowerMW, Range{43, 49}, b.state.seed^1, now)
	o.ObserveFloat64(ins.blockAC, ac, metric.WithAttributes(base...))

	pr := sc.RampedValue(sel, MetricBlockPerformanceRatio, Range{0.94, 0.985}, b.state.seed^2, now)
	o.ObserveFloat64(ins.blockPR, clamp(pr, 0, 1.05), metric.WithAttributes(base...))

	b.state.cumEnergyMWh += ac * dt / 3600
	o.ObserveFloat64(ins.blockEnergy, b.state.cumEnergyMWh, metric.WithAttributes(base...))

	avail := sc.RampedValue(sel, MetricBlockAvailability, Range{0.985, 1.0}, b.state.seed^3, now)
	o.ObserveFloat64(ins.blockAvail, clamp(avail, 0, 1), metric.WithAttributes(base...))
	_ = c
}

func observeStation(o metric.Observer, ins *instruments, s *InverterStation, sc *Scenario, now time.Time) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	sel := selectorStation(s.ID)
	base := stationAttrs(s)

	ac := sc.RampedValue(sel, MetricInverterStationACPowerKW, Range{10500, 12200}, s.state.seed^1, now)
	o.ObserveFloat64(ins.isACPower, ac, metric.WithAttributes(base...))

	volt := sc.RampedValue(sel, MetricInverterStationACVoltageV, Range{595, 605}, s.state.seed^2, now)
	o.ObserveFloat64(ins.isACVoltage, volt, metric.WithAttributes(base...))

	cur := ac * 1000 / (math.Sqrt(3) * volt)
	o.ObserveFloat64(ins.isACCurrent, cur, metric.WithAttributes(base...))

	hz := sc.RampedValue(sel, MetricInverterStationGridHz, Range{49.95, 50.05}, s.state.seed^3, now)
	o.ObserveFloat64(ins.isGridHz, hz, metric.WithAttributes(base...))
}

func observeInverter(o metric.Observer, ins *instruments, inv *Inverter, sc *Scenario, now time.Time) {
	inv.state.mu.Lock()
	defer inv.state.mu.Unlock()
	dt := dtAdvance(&inv.state.lastTick, now)
	sel := selectorInverter(inv.ID)
	base := inverterAttrs(inv)

	dc := sc.RampedValue(sel, MetricInverterDCPowerKW, Range{2800, 3050}, inv.state.seed^1, now)
	o.ObserveFloat64(ins.invDC, dc, metric.WithAttributes(base...))

	ac := sc.RampedValue(sel, MetricInverterACPowerKW, Range{2720, 2980}, inv.state.seed^2, now)
	o.ObserveFloat64(ins.invAC, ac, metric.WithAttributes(base...))

	eff := sc.RampedValue(sel, MetricInverterEfficiencyPct, Range{97.0, 98.5}, inv.state.seed^3, now)
	o.ObserveFloat64(ins.invEff, clamp(eff, 0, 100), metric.WithAttributes(base...))

	o.ObserveFloat64(ins.invInternalTemp,
		sc.RampedValue(sel, MetricInverterInternalTempC, Range{38, 52}, inv.state.seed^4, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invIGBTTemp,
		sc.RampedValue(sel, MetricInverterIGBTTempC, Range{52, 70}, inv.state.seed^5, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invHeatsinkTemp,
		sc.RampedValue(sel, MetricInverterHeatsinkTempC, Range{42, 58}, inv.state.seed^6, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invFanRPM,
		sc.RampedValue(sel, MetricInverterCoolingFanRPM, Range{2200, 2800}, inv.state.seed^7, now),
		metric.WithAttributes(base...))

	o.ObserveFloat64(ins.invMPPTV,
		sc.RampedValue(sel, MetricInverterMPPTVoltageV, Range{860, 920}, inv.state.seed^8, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invMPPTI,
		sc.RampedValue(sel, MetricInverterMPPTCurrentA, Range{3.0, 3.8}, inv.state.seed^9, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invDCBusV,
		sc.RampedValue(sel, MetricInverterDCBusVoltageV, Range{1140, 1180}, inv.state.seed^10, now),
		metric.WithAttributes(base...))

	derate := sc.RampedValue(sel, MetricInverterDerateState, Range{0, 0}, inv.state.seed^11, now)
	o.ObserveFloat64(ins.invDerateState, math.Round(derate), metric.WithAttributes(base...))

	inv.state.cumEnergyKWh += ac * dt / 3600
	o.ObserveFloat64(ins.invEnergy, inv.state.cumEnergyKWh, metric.WithAttributes(base...))

	o.ObserveFloat64(ins.invTHD,
		sc.RampedValue(sel, MetricInverterTHDPct, Range{0.8, 2.4}, inv.state.seed^12, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invStringImbal,
		sc.RampedValue(sel, MetricInverterStringImbalancePct, Range{0.4, 2.2}, inv.state.seed^13, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.invStringMinCur,
		sc.RampedValue(sel, MetricInverterStringMinCurrentA, Range{3.0, 3.7}, inv.state.seed^14, now),
		metric.WithAttributes(base...))
}

func observeTransformer(o metric.Observer, ins *instruments, t *MVTransformer, sc *Scenario, now time.Time) {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	sel := selectorTransformer(t.ID)
	base := transformerAttrs(t)

	o.ObserveFloat64(ins.trafoWinding,
		sc.RampedValue(sel, MetricMVTrafoWindingTempC, Range{62, 76}, t.state.seed^1, now),
		metric.WithAttributes(append(base, attribute.String("winding", "HV"))...))
	o.ObserveFloat64(ins.trafoWinding,
		sc.RampedValue(sel, MetricMVTrafoWindingTempC, Range{58, 72}, t.state.seed^11, now)*0.94,
		metric.WithAttributes(append(base, attribute.String("winding", "LV"))...))

	o.ObserveFloat64(ins.trafoOil,
		sc.RampedValue(sel, MetricMVTrafoOilTempC, Range{45, 60}, t.state.seed^2, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trafoOilLevel,
		sc.RampedValue(sel, MetricMVTrafoOilLevelPct, Range{96, 99.5}, t.state.seed^3, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trafoOilFlow,
		sc.RampedValue(sel, MetricMVTrafoOilFlowLPM, Range{70, 85}, t.state.seed^4, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trafoRadiator,
		sc.RampedValue(sel, MetricMVTrafoRadiatorTempC, Range{42, 56}, t.state.seed^5, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trafoLoad,
		sc.RampedValue(sel, MetricMVTrafoLoadKVA, Range{48000, 62000}, t.state.seed^6, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trafoTap, 9, metric.WithAttributes(base...))
	buch := 0.0
	if sc.IsActiveOn(sel, MetricMVTrafoWindingTempC, now) && t.IncidentRole == rolePrimaryTransformer {
		buch = 1
	}
	o.ObserveFloat64(ins.trafoBuchholz, buch, metric.WithAttributes(base...))
}

func observeCompound(o metric.Observer, ins *instruments, mc *MVCompound, sc *Scenario, now time.Time) {
	sel := selectorCompound(mc.ID)
	base := compoundAttrs(mc)
	o.ObserveFloat64(ins.compoundAmbient,
		sc.RampedValue(sel, MetricMVCompoundAmbientTempC, Range{34, 42}, seedFor(mc.ID)^1, now),
		metric.WithAttributes(base...))
}

func observeTracker(o metric.Observer, ins *instruments, t *Tracker, sc *Scenario, now time.Time) {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	dt := dtAdvance(&t.state.lastTick, now)
	sel := selectorTracker(t.ID)
	base := trackerAttrs(t)
	o.ObserveFloat64(ins.trkAngle,
		sc.RampedValue(sel, MetricTrackerAngleDeg, Range{40, 50}, t.state.seed^1, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trkTarget,
		sc.RampedValue(sel, MetricTrackerTargetAngleDeg, Range{40, 50}, t.state.seed^2, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.trkMotorCur,
		sc.RampedValue(sel, MetricTrackerMotorCurrentA, Range{2.4, 4.2}, t.state.seed^3, now),
		metric.WithAttributes(base...))
	faultRate := sc.RampedValue(sel, MetricTrackerFaultCount, Range{0, 0.02}, t.state.seed^4, now)
	t.state.cumFault += faultRate * dt
	o.ObserveFloat64(ins.trkFaultCnt, t.state.cumFault, metric.WithAttributes(base...))
}

func observeMet(o metric.Observer, ins *instruments, m *MetStation, sc *Scenario, now time.Time) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	sel := selectorMet(m.ID)
	base := metAttrs(m)
	o.ObserveFloat64(ins.metPOA,
		sc.RampedValue(sel, MetricMetIrradiancePOAWm2, Range{820, 940}, m.state.seed^1, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metGHI,
		sc.RampedValue(sel, MetricMetIrradianceGHIWm2, Range{780, 900}, m.state.seed^2, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metAmb,
		sc.RampedValue(sel, MetricMetAmbientTempC, Range{32, 41}, m.state.seed^3, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metMod,
		sc.RampedValue(sel, MetricMetModuleTempC, Range{45, 58}, m.state.seed^4, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metWind,
		sc.RampedValue(sel, MetricMetWindSpeedMPS, Range{2.0, 6.0}, m.state.seed^5, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metHumid,
		sc.RampedValue(sel, MetricMetHumidityPct, Range{18, 36}, m.state.seed^6, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.metSoil,
		sc.RampedValue(sel, MetricMetSoilingLossPct, Range{0.4, 2.2}, m.state.seed^7, now),
		metric.WithAttributes(base...))
}

func observeSubstation(o metric.Observer, ins *instruments, s *Substation, sc *Scenario, now time.Time) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	dt := dtAdvance(&s.state.lastTick, now)
	sel := selectorSubstation(s.ID)
	base := substationAttrs(s)
	export := sc.RampedValue(sel, MetricSubstationExportPowerMW, Range{255, 285}, s.state.seed^1, now)
	o.ObserveFloat64(ins.subExport, export, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.subHz,
		sc.RampedValue(sel, MetricSubstationGridFrequencyHz, Range{49.95, 50.05}, s.state.seed^2, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.subBusKv,
		sc.RampedValue(sel, MetricSubstationBusbarVoltageKV, Range{218, 222}, s.state.seed^3, now),
		metric.WithAttributes(base...))
	s.state.cumExportMWh += export * dt / 3600
	o.ObserveFloat64(ins.subEnergy, s.state.cumExportMWh, metric.WithAttributes(base...))
}

// observeAlerts emits cardinal_alert_active gauges for synthetic alerts
// that fire during a profile. Without a profile this emits nothing.
func observeAlerts(o metric.Observer, ins *instruments, sc *Scenario, _ time.Time) {
	id := sc.ActiveProfileID()
	if id == "" {
		return
	}
	switch id {
	case ProfileMVTransformerWindingOverheat:
		o.ObserveFloat64(ins.alertActive, 1, metric.WithAttributes(
			attribute.String("alert_name", "MV transformer winding temp high"),
			attribute.String("alert_severity", "high"),
			attribute.String("affected_entity_type", "mv_transformer"),
			attribute.String("affected_entity_id", mvTrafoT04A),
			attribute.String("mv_transformer_id", mvTrafoT04A),
			attribute.String("mv_compound_id", mvCompound04),
			attribute.String("suspected_layer", "mv_transformer"),
		))
		o.ObserveFloat64(ins.alertActive, 1, metric.WithAttributes(
			attribute.String("alert_name", "PPA dispatch deviation breach"),
			attribute.String("alert_severity", "critical"),
			attribute.String("affected_entity_type", "offtaker"),
			attribute.String("affected_entity_id", offtakerSECI),
			attribute.String("offtaker_id", offtakerSECI),
			attribute.String("suspected_layer", "mv_transformer"),
		))
	case ProfileInverterCoolingFault:
		o.ObserveFloat64(ins.alertActive, 1, metric.WithAttributes(
			attribute.String("alert_name", "Inverter cooling fan fault"),
			attribute.String("alert_severity", "high"),
			attribute.String("affected_entity_type", "inverter"),
			attribute.String("affected_entity_id", "INV-08-02-03"),
			attribute.String("inverter_id", "INV-08-02-03"),
			attribute.String("inverter_station_id", "IS-08-02"),
			attribute.String("block_id", "block-08"),
			attribute.String("suspected_layer", "inverter"),
		))
	case ProfileTrackerStowMisalignment:
		o.ObserveFloat64(ins.alertActive, 1, metric.WithAttributes(
			attribute.String("alert_name", "Tracker stow misalignment"),
			attribute.String("alert_severity", "high"),
			attribute.String("affected_entity_type", "tracker"),
			attribute.String("affected_entity_id", "TRK-12"),
			attribute.String("tracker_id", "TRK-12"),
			attribute.String("block_id", "block-12"),
			attribute.String("suspected_layer", "tracker"),
		))
	case ProfileStringPIDDegradation:
		o.ObserveFloat64(ins.alertActive, 1, metric.WithAttributes(
			attribute.String("alert_name", "Inverter string imbalance"),
			attribute.String("alert_severity", "warning"),
			attribute.String("affected_entity_type", "inverter"),
			attribute.String("affected_entity_id", "INV-10-03-01"),
			attribute.String("inverter_id", "INV-10-03-01"),
			attribute.String("inverter_station_id", "IS-10-03"),
			attribute.String("block_id", "block-10"),
			attribute.String("suspected_layer", "string"),
		))
	}
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// registerAll creates every async instrument.
func registerAll(m metric.Meter, ins *instruments) error {
	var err error
	g := func(name, desc string) metric.Float64ObservableGauge {
		if err != nil {
			return nil
		}
		var x metric.Float64ObservableGauge
		x, err = m.Float64ObservableGauge(name, metric.WithDescription(desc))
		return x
	}
	c := func(name, desc string) metric.Float64ObservableCounter {
		if err != nil {
			return nil
		}
		var x metric.Float64ObservableCounter
		x, err = m.Float64ObservableCounter(name, metric.WithDescription(desc))
		return x
	}

	ins.ppaScheduled = g(MetricPPAScheduledDispatchMW, "PPA day-ahead scheduled dispatch MW")
	ins.ppaActual = g(MetricPPAActualDispatchMW, "PPA actual dispatch MW")
	ins.ppaDeviation = g(MetricPPADispatchDeviationPct, "PPA dispatch deviation percent")
	ins.ppaCompliance = g(MetricPPAComplianceRatio, "PPA compliance ratio (1.0 = on schedule)")
	ins.ppaBurn = g(MetricPPABurnRate, "PPA burn rate (compliance error budget)")
	ins.ppaRevAtRisk = g(MetricPPARevenueAtRiskINRMin, "PPA revenue at risk INR per minute")

	ins.blockAC = g(MetricBlockACPowerMW, "Block AC power MW")
	ins.blockPR = g(MetricBlockPerformanceRatio, "Block performance ratio")
	ins.blockEnergy = c(MetricBlockEnergyMWh, "Block cumulative energy MWh")
	ins.blockAvail = g(MetricBlockAvailability, "Block availability ratio")

	ins.isACPower = g(MetricInverterStationACPowerKW, "Inverter station AC power kW")
	ins.isACVoltage = g(MetricInverterStationACVoltageV, "Inverter station AC voltage V")
	ins.isACCurrent = g(MetricInverterStationACCurrentA, "Inverter station AC current A")
	ins.isGridHz = g(MetricInverterStationGridHz, "Inverter station grid frequency Hz")

	ins.invDC = g(MetricInverterDCPowerKW, "Inverter DC power kW")
	ins.invAC = g(MetricInverterACPowerKW, "Inverter AC power kW")
	ins.invEff = g(MetricInverterEfficiencyPct, "Inverter efficiency percent")
	ins.invInternalTemp = g(MetricInverterInternalTempC, "Inverter internal temperature C")
	ins.invIGBTTemp = g(MetricInverterIGBTTempC, "Inverter IGBT junction temperature C")
	ins.invHeatsinkTemp = g(MetricInverterHeatsinkTempC, "Inverter heatsink temperature C")
	ins.invFanRPM = g(MetricInverterCoolingFanRPM, "Inverter cooling fan RPM")
	ins.invMPPTV = g(MetricInverterMPPTVoltageV, "Inverter MPPT voltage V")
	ins.invMPPTI = g(MetricInverterMPPTCurrentA, "Inverter MPPT current A")
	ins.invDCBusV = g(MetricInverterDCBusVoltageV, "Inverter DC bus voltage V")
	ins.invDerateState = g(MetricInverterDerateState, "Inverter derate state (0=normal,1=derate,2=trip)")
	ins.invEnergy = c(MetricInverterEnergyKWh, "Inverter cumulative energy kWh")
	ins.invTHD = g(MetricInverterTHDPct, "Inverter total harmonic distortion percent")
	ins.invStringImbal = g(MetricInverterStringImbalancePct, "Inverter string current imbalance percent")
	ins.invStringMinCur = g(MetricInverterStringMinCurrentA, "Inverter weakest string DC current A")

	ins.trafoWinding = g(MetricMVTrafoWindingTempC, "MV transformer winding temperature C")
	ins.trafoOil = g(MetricMVTrafoOilTempC, "MV transformer oil temperature C")
	ins.trafoOilLevel = g(MetricMVTrafoOilLevelPct, "MV transformer oil level percent")
	ins.trafoOilFlow = g(MetricMVTrafoOilFlowLPM, "MV transformer cooling oil flow LPM")
	ins.trafoRadiator = g(MetricMVTrafoRadiatorTempC, "MV transformer radiator temperature C")
	ins.trafoLoad = g(MetricMVTrafoLoadKVA, "MV transformer load kVA")
	ins.trafoTap = g(MetricMVTrafoTapPosition, "MV transformer tap position")
	ins.trafoBuchholz = g(MetricMVTrafoBuchholzAlarm, "MV transformer Buchholz relay alarm")

	ins.compoundAmbient = g(MetricMVCompoundAmbientTempC, "MV compound outdoor ambient temperature C")

	ins.trkAngle = g(MetricTrackerAngleDeg, "Tracker current angle degrees")
	ins.trkTarget = g(MetricTrackerTargetAngleDeg, "Tracker target angle degrees")
	ins.trkMotorCur = g(MetricTrackerMotorCurrentA, "Tracker motor current A")
	ins.trkFaultCnt = c(MetricTrackerFaultCount, "Tracker cumulative fault count")

	ins.metPOA = g(MetricMetIrradiancePOAWm2, "Met station POA irradiance W/m²")
	ins.metGHI = g(MetricMetIrradianceGHIWm2, "Met station GHI irradiance W/m²")
	ins.metAmb = g(MetricMetAmbientTempC, "Met station ambient temperature C")
	ins.metMod = g(MetricMetModuleTempC, "Met station module backsheet temperature C")
	ins.metWind = g(MetricMetWindSpeedMPS, "Met station wind speed m/s")
	ins.metHumid = g(MetricMetHumidityPct, "Met station relative humidity percent")
	ins.metSoil = g(MetricMetSoilingLossPct, "Met station soiling loss percent")

	ins.subExport = g(MetricSubstationExportPowerMW, "Substation export MW")
	ins.subHz = g(MetricSubstationGridFrequencyHz, "Substation grid frequency Hz")
	ins.subBusKv = g(MetricSubstationBusbarVoltageKV, "Substation busbar voltage kV")
	ins.subEnergy = c(MetricSubstationEnergyMWh, "Substation cumulative export energy MWh")

	ins.alertActive = g(MetricCardinalAlertActive, "Active synthetic alert state")

	if err != nil {
		return fmt.Errorf("create solar instruments: %w", err)
	}
	return nil
}
