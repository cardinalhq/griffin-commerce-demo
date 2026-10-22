#!/usr/bin/env python3
"""
Authors the four Adani Khavda solar farm demo dashboards against
maestro. Mirrors the airtel-prod author.py pattern.

Targets the Cardinal HQ org (c4375e34-…) in
prod-us-east-2-global — that's where the aws-prod-us-east-2-global
node-local collector routes telemetry. The natural-sounding "Adani"
org doesn't exist; until the collector is wired to a different org,
telemetry surfaces in Cardinal HQ. Override by editing ORG_ID.

Dashboards follow the spec catalog (§24):
  1. Plant Health Overview       — fleet posture, PPA SLO, block rollup, alerts
  2. Block Detail                 — inverter stations, inverters, strings, trackers, met
  3. Electrical Infrastructure   — MV transformers, compound ambient, substation
  4. Correlation & Blast Radius  — cause-to-symptom chain, at-risk siblings, alerts

Schema reference: ui-pages/src/dashboards/v2/types.ts. 24-column grid,
cells carry {x,y,w,h,i}. Panels are 'label' (single-stat tile) or
'timeseries' (variant: line | stacked-bar | stacked-area). Variables drive
`$offtaker_id` / `$block_id` / `$mv_transformer_id` / `$mv_compound_id` /
`$inverter_station_id` / `$inverter_id` interpolation — these labels exist
on the simulator's emitted series so `correlate_dashboards` reaches them
in one hop.
"""
import json

ORG_ID = "c4375e34-dfcf-498a-8ba3-a02d119baf82"

# ---------- helpers ----------

def label_panel(pid, title, query, color="#3b82f6", unit="", reducer="last"):
    p = {
        "id": pid, "kind": "label", "title": title, "color": color,
        "queries": [{"query": query, "queryKind": "prometheus"}],
        "reducer": reducer,
    }
    if unit:
        p["format"] = {"unit": unit}
    return p

def ts_panel(pid, title, queries, variant="line"):
    if isinstance(queries, str):
        queries = [{"query": queries, "queryKind": "prometheus"}]
    elif queries and isinstance(queries[0], str):
        queries = [{"query": q, "queryKind": "prometheus"} for q in queries]
    return {
        "id": pid, "kind": "timeseries", "title": title,
        "queries": queries, "variant": variant,
    }

def row_at(y, *cells):
    out = []
    x = 0
    for pid, w, h in cells:
        out.append({"i": pid, "x": x, "y": y, "w": w, "h": h})
        x += w
    return out

# ---------- variables ----------

VAR_OFFTAKER = {
    "kind": "query", "name": "offtaker_id", "label": "Offtaker (PPA)",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "offtaker_id", "metric": "ppa_dispatch_deviation_pct", "signal": "metrics"},
    "defaultValue": ["seci_phase_iii"],
}

VAR_BLOCK = {
    "kind": "query", "name": "block_id", "label": "Block",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "block_id", "metric": "block_performance_ratio", "signal": "metrics"},
    "defaultValue": ["block-04"],
}

VAR_STATION = {
    "kind": "query", "name": "inverter_station_id", "label": "Inverter Station (PCS)",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "inverter_station_id", "metric": "inverter_station_ac_power_kw", "signal": "metrics"},
    "defaultValue": ["IS-04-01"],
}

VAR_INVERTER = {
    "kind": "query", "name": "inverter_id", "label": "Inverter",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "inverter_id", "metric": "inverter_ac_power_kw", "signal": "metrics"},
    "defaultValue": ["INV-04-01-01"],
}

VAR_TRANSFORMER = {
    "kind": "query", "name": "mv_transformer_id", "label": "MV Transformer",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "mv_transformer_id", "metric": "mv_transformer_winding_temp_c", "signal": "metrics"},
    "defaultValue": ["T-04-A"],
}

VAR_COMPOUND = {
    "kind": "query", "name": "mv_compound_id", "label": "MV Compound",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "mv_compound_id", "metric": "mv_compound_ambient_temp_c", "signal": "metrics"},
    "defaultValue": ["mvc-04"],
}

VAR_TRACKER = {
    "kind": "query", "name": "tracker_id", "label": "Tracker",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "tracker_id", "metric": "tracker_angle_deg", "signal": "metrics"},
    "defaultValue": ["TRK-12"],
}

VAR_MET = {
    "kind": "query", "name": "met_station_id", "label": "Met Station",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "met_station_id", "metric": "met_irradiance_poa_w_m2", "signal": "metrics"},
    "defaultValue": ["MET-04-1"],
}

# ============================================================
# 1. Plant Health Overview — spec §24.1
# ============================================================

def dashboard_plant_health():
    """Plant ops view — what an Adani Renewable Ops engineer pulls up
    every morning. Reads as steady-state fleet health most of the time,
    with the PPA breach narrative one of many signals on the page."""
    panels = {}

    # Fleet KPI strip (4 tiles).
    panels["ph_kpi_offtakers"] = label_panel("ph_kpi_offtakers", "Offtakers (PPAs)",
        'count(max by (offtaker_id)(ppa_dispatch_deviation_pct))', color="#3b82f6")
    panels["ph_kpi_blocks"]    = label_panel("ph_kpi_blocks", "Blocks Online",
        'count(max by (block_id)(block_performance_ratio))', color="#3b82f6")
    panels["ph_kpi_inverters"] = label_panel("ph_kpi_inverters", "Inverters",
        'count(max by (inverter_id)(inverter_ac_power_kw))', color="#3b82f6")
    panels["ph_kpi_breach"]    = label_panel("ph_kpi_breach", "Offtakers Breaching PPA",
        'sum(max by (offtaker_id)(ppa_dispatch_deviation_pct) > bool 8)', color="#ef4444")

    # Selected offtaker deep tile row (4) — drives the rest of the page.
    panels["ph_burn"]     = label_panel("ph_burn", "PPA Burn Rate",
        'max(ppa_burn_rate{offtaker_id="$offtaker_id"})', color="#ef4444", unit="x")
    panels["ph_dev"]      = label_panel("ph_dev", "Dispatch Deviation",
        'max(ppa_dispatch_deviation_pct{offtaker_id="$offtaker_id"})', color="#f59e0b", unit="%")
    panels["ph_comp"]     = label_panel("ph_comp", "PPA Compliance",
        'min(ppa_compliance_ratio{offtaker_id="$offtaker_id"}) * 100', color="#10b981", unit="%")
    panels["ph_revrisk"]  = label_panel("ph_revrisk", "Revenue At Risk",
        'max(ppa_revenue_at_risk_inr_per_min{offtaker_id="$offtaker_id"})', color="#f59e0b", unit="INR/min")

    # Plant production strip (4)
    panels["ph_site_mw"]   = label_panel("ph_site_mw", "Substation Export",
        'max(substation_export_power_mw)', color="#3b82f6", unit="MW")
    panels["ph_site_hz"]   = label_panel("ph_site_hz", "Grid Frequency",
        'avg(substation_grid_frequency_hz)', color="#10b981", unit="Hz")
    panels["ph_site_irr"]  = label_panel("ph_site_irr", "POA Irradiance",
        'avg(met_irradiance_poa_w_m2)', color="#3b82f6", unit="W/m²")
    panels["ph_site_amb"]  = label_panel("ph_site_amb", "Ambient Temp",
        'avg(met_ambient_temp_c)', color="#f59e0b", unit="°C")

    # Fleet PPA trends (2) — the wall every morning
    panels["ph_dev_all"]    = ts_panel("ph_dev_all", "Dispatch Deviation by Offtaker (%)",
        'max by (offtaker_id)(ppa_dispatch_deviation_pct)')
    panels["ph_burn_all"]   = ts_panel("ph_burn_all", "PPA Burn Rate by Offtaker",
        'max by (offtaker_id)(ppa_burn_rate)')

    # Fleet PR + power (2)
    panels["ph_pr_block"]   = ts_panel("ph_pr_block", "Performance Ratio by Block",
        'avg by (block_id)(block_performance_ratio)')
    panels["ph_mw_block"]   = ts_panel("ph_mw_block", "AC Power by Block (MW)",
        'max by (block_id)(block_ac_power_mw)', variant="stacked-area")

    # Inverter station rollup (3)
    panels["ph_is_pwr"]     = ts_panel("ph_is_pwr", "Inverter Station AC Power (kW)",
        'max by (inverter_station_id)(inverter_station_ac_power_kw)')
    panels["ph_is_volt"]    = ts_panel("ph_is_volt", "Inverter Station AC Voltage (V)",
        'avg by (inverter_station_id)(inverter_station_ac_voltage_v)')
    panels["ph_is_hz"]      = ts_panel("ph_is_hz", "Inverter Station Grid Frequency",
        'avg by (inverter_station_id)(inverter_station_grid_frequency_hz)')

    # Weather row (3)
    panels["ph_poa"]    = ts_panel("ph_poa", "POA Irradiance by Met Station (W/m²)",
        'max by (met_station_id)(met_irradiance_poa_w_m2)')
    panels["ph_mod"]    = ts_panel("ph_mod", "Module Temperature by Met Station (°C)",
        'max by (met_station_id)(met_module_temp_c)')
    panels["ph_soil"]   = ts_panel("ph_soil", "Soiling Loss by Met Station (%)",
        'max by (met_station_id)(met_soiling_loss_pct)')

    # Shared electrical infrastructure rollup (3)
    panels["ph_trafo_w"]  = ts_panel("ph_trafo_w", "MV Transformer Winding Temp by Trafo (°C)",
        'max by (mv_transformer_id, winding)(mv_transformer_winding_temp_c)')
    panels["ph_trafo_l"]  = ts_panel("ph_trafo_l", "MV Transformer Load by Trafo (kVA)",
        'max by (mv_transformer_id)(mv_transformer_load_kva)')
    panels["ph_compound"] = ts_panel("ph_compound", "MV Compound Ambient Temp (°C)",
        'max by (mv_compound_id)(mv_compound_ambient_temp_c)')

    # Active alerts
    panels["ph_alerts"]   = ts_panel("ph_alerts", "Active Alerts",
        'max by (alert_name, alert_severity, affected_entity_id, suspected_layer)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "Fleet Posture", "cells": row_at(0,
            ("ph_kpi_offtakers", 6, 3), ("ph_kpi_blocks", 6, 3),
            ("ph_kpi_inverters", 6, 3), ("ph_kpi_breach", 6, 3))},
        {"title": "Selected Offtaker — PPA Snapshot", "cells": row_at(0,
            ("ph_burn", 6, 3), ("ph_dev", 6, 3), ("ph_comp", 6, 3), ("ph_revrisk", 6, 3))},
        {"title": "Plant Production", "cells": row_at(0,
            ("ph_site_mw", 6, 3), ("ph_site_hz", 6, 3),
            ("ph_site_irr", 6, 3), ("ph_site_amb", 6, 3))},
        {"title": "Fleet PPA Trends", "cells": row_at(0,
            ("ph_dev_all", 12, 7), ("ph_burn_all", 12, 7))},
        {"title": "Block Production", "cells": row_at(0,
            ("ph_pr_block", 12, 7), ("ph_mw_block", 12, 7))},
        {"title": "Inverter Station Rollup", "cells": row_at(0,
            ("ph_is_pwr", 8, 6), ("ph_is_volt", 8, 6), ("ph_is_hz", 8, 6))},
        {"title": "Weather / Met", "cells": row_at(0,
            ("ph_poa", 8, 6), ("ph_mod", 8, 6), ("ph_soil", 8, 6))},
        {"title": "Shared Electrical Infrastructure", "cells": row_at(0,
            ("ph_trafo_w", 8, 6), ("ph_trafo_l", 8, 6), ("ph_compound", 8, 6))},
        {"title": "Active Alerts", "cells": row_at(0,
            ("ph_alerts", 24, 6))},
    ]
    return {
        "name": "Adani — Plant Health Overview",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_OFFTAKER],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 2. Block Detail — spec §24.2
# ============================================================

def dashboard_block_detail():
    """Block ops view — what an O&M engineer pulls up to triage a single
    block. Variable-driven on $block_id; everything zooms to that one
    50 MW unit."""
    panels = {}

    # Tile strip (6)
    panels["bd_mw"]    = label_panel("bd_mw", "Block AC Power",
        'max(block_ac_power_mw{block_id="$block_id"})', color="#3b82f6", unit="MW")
    panels["bd_pr"]    = label_panel("bd_pr", "Performance Ratio",
        'avg(block_performance_ratio{block_id="$block_id"}) * 100', color="#10b981", unit="%")
    panels["bd_avail"] = label_panel("bd_avail", "Availability",
        'min(block_availability_ratio{block_id="$block_id"}) * 100', color="#10b981", unit="%")
    panels["bd_poa"]   = label_panel("bd_poa", "POA Irradiance",
        'avg(met_irradiance_poa_w_m2{block_id="$block_id"})', color="#3b82f6", unit="W/m²")
    panels["bd_mod"]   = label_panel("bd_mod", "Module Temp",
        'max(met_module_temp_c{block_id="$block_id"})', color="#f59e0b", unit="°C")
    panels["bd_soil"]  = label_panel("bd_soil", "Soiling Loss",
        'max(met_soiling_loss_pct{block_id="$block_id"})', color="#f59e0b", unit="%")

    # Inverter stations in the block (3)
    panels["bd_is_pwr"]  = ts_panel("bd_is_pwr", "PCS AC Power (kW)",
        'max by (inverter_station_id)(inverter_station_ac_power_kw{block_id="$block_id"})')
    panels["bd_is_volt"] = ts_panel("bd_is_volt", "PCS AC Voltage (V)",
        'avg by (inverter_station_id)(inverter_station_ac_voltage_v{block_id="$block_id"})')
    panels["bd_is_cur"]  = ts_panel("bd_is_cur", "PCS AC Current (A)",
        'avg by (inverter_station_id)(inverter_station_ac_current_a{block_id="$block_id"})')

    # Inverter heat strip (3) — distinguishes the bad one
    panels["bd_inv_pwr"]  = ts_panel("bd_inv_pwr", "Inverter AC Power per Unit (kW)",
        'max by (inverter_id)(inverter_ac_power_kw{block_id="$block_id"})')
    panels["bd_inv_eff"]  = ts_panel("bd_inv_eff", "Inverter Efficiency (%)",
        'min by (inverter_id)(inverter_efficiency_pct{block_id="$block_id"})')
    panels["bd_inv_temp"] = ts_panel("bd_inv_temp", "Inverter IGBT Temp (°C)",
        'max by (inverter_id)(inverter_igbt_temp_c{block_id="$block_id"})')

    # Inverter cooling + derate (3)
    panels["bd_inv_fan"]    = ts_panel("bd_inv_fan", "Inverter Cooling Fan RPM",
        'min by (inverter_id)(inverter_cooling_fan_rpm{block_id="$block_id"})')
    panels["bd_inv_heat"]   = ts_panel("bd_inv_heat", "Inverter Heatsink Temp (°C)",
        'max by (inverter_id)(inverter_heatsink_temp_c{block_id="$block_id"})')
    panels["bd_inv_derate"] = ts_panel("bd_inv_derate", "Inverter Derate State",
        'max by (inverter_id)(inverter_derate_state{block_id="$block_id"})',
        variant="stacked-bar")

    # Strings / MPPT (3) — find a PID-prone inverter
    panels["bd_mppt_v"]    = ts_panel("bd_mppt_v", "MPPT Voltage (V)",
        'avg by (inverter_id)(inverter_mppt_voltage_v{block_id="$block_id"})')
    panels["bd_mppt_i"]    = ts_panel("bd_mppt_i", "MPPT Current (A)",
        'avg by (inverter_id)(inverter_mppt_current_a{block_id="$block_id"})')
    panels["bd_imbal"]     = ts_panel("bd_imbal", "String Current Imbalance (%)",
        'max by (inverter_id)(inverter_string_imbalance_pct{block_id="$block_id"})')

    # Tracker + met for the block (3)
    panels["bd_trk_angle"] = ts_panel("bd_trk_angle", "Tracker Angle vs Target (°)",
        ['avg by (tracker_id)(tracker_angle_deg{block_id="$block_id"})',
         'avg by (tracker_id)(tracker_target_angle_deg{block_id="$block_id"})'])
    panels["bd_trk_cur"]   = ts_panel("bd_trk_cur", "Tracker Motor Current (A)",
        'max by (tracker_id)(tracker_motor_current_a{block_id="$block_id"})')
    panels["bd_trk_fault"] = ts_panel("bd_trk_fault", "Tracker Faults (rate)",
        'rate(tracker_fault_count_total{block_id="$block_id"}[5m])')

    # Met (3)
    panels["bd_irr"]   = ts_panel("bd_irr", "POA vs GHI by Met (W/m²)",
        ['max by (met_station_id)(met_irradiance_poa_w_m2{block_id="$block_id"})',
         'max by (met_station_id)(met_irradiance_ghi_w_m2{block_id="$block_id"})'])
    panels["bd_wind"]  = ts_panel("bd_wind", "Wind Speed (m/s)",
        'avg by (met_station_id)(met_wind_speed_mps{block_id="$block_id"})')
    panels["bd_humid"] = ts_panel("bd_humid", "Humidity (%)",
        'avg by (met_station_id)(met_humidity_pct{block_id="$block_id"})')

    sections = [
        {"title": "Block Posture", "cells": row_at(0,
            ("bd_mw", 4, 3), ("bd_pr", 4, 3), ("bd_avail", 4, 3),
            ("bd_poa", 4, 3), ("bd_mod", 4, 3), ("bd_soil", 4, 3))},
        {"title": "Inverter Stations (PCS)", "cells": row_at(0,
            ("bd_is_pwr", 8, 6), ("bd_is_volt", 8, 6), ("bd_is_cur", 8, 6))},
        {"title": "Inverter Production & Thermal", "cells": row_at(0,
            ("bd_inv_pwr", 8, 7), ("bd_inv_eff", 8, 7), ("bd_inv_temp", 8, 7))},
        {"title": "Inverter Cooling & Derate", "cells": row_at(0,
            ("bd_inv_fan", 8, 6), ("bd_inv_heat", 8, 6), ("bd_inv_derate", 8, 6))},
        {"title": "Strings / MPPT — find PID candidates", "cells": row_at(0,
            ("bd_mppt_v", 8, 6), ("bd_mppt_i", 8, 6), ("bd_imbal", 8, 6))},
        {"title": "Trackers", "cells": row_at(0,
            ("bd_trk_angle", 8, 6), ("bd_trk_cur", 8, 6), ("bd_trk_fault", 8, 6))},
        {"title": "Met Station", "cells": row_at(0,
            ("bd_irr", 8, 6), ("bd_wind", 8, 6), ("bd_humid", 8, 6))},
    ]
    return {
        "name": "Adani — Block Detail",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_BLOCK, VAR_STATION, VAR_INVERTER, VAR_TRACKER, VAR_MET],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 3. Electrical Infrastructure — spec §24.3
# ============================================================

def dashboard_electrical_infra():
    """Substation engineer's view — MV transformers (the canonical
    failure surface), compound ambient, and the export bus. Variable
    drives $mv_transformer_id."""
    panels = {}

    # Inventory + KPIs (4)
    panels["ei_trafos"]   = label_panel("ei_trafos", "MV Transformers",
        'count(max by (mv_transformer_id)(mv_transformer_winding_temp_c))', color="#3b82f6")
    panels["ei_compounds"] = label_panel("ei_compounds", "MV Compounds",
        'count(max by (mv_compound_id)(mv_compound_ambient_temp_c))', color="#3b82f6")
    panels["ei_export"]   = label_panel("ei_export", "Total Export",
        'max(substation_export_power_mw)', color="#3b82f6", unit="MW")
    panels["ei_hz"]       = label_panel("ei_hz", "Grid Frequency",
        'avg(substation_grid_frequency_hz)', color="#10b981", unit="Hz")

    # Selected transformer tile strip (6)
    panels["ei_t_winding"] = label_panel("ei_t_winding", "Winding Temp (HV)",
        'max(mv_transformer_winding_temp_c{mv_transformer_id="$mv_transformer_id", winding="HV"})',
        color="#ef4444", unit="°C")
    panels["ei_t_oil"]     = label_panel("ei_t_oil", "Oil Temp",
        'max(mv_transformer_oil_temp_c{mv_transformer_id="$mv_transformer_id"})',
        color="#f59e0b", unit="°C")
    panels["ei_t_flow"]    = label_panel("ei_t_flow", "Cooling Oil Flow",
        'min(mv_transformer_cooling_oil_flow_lpm{mv_transformer_id="$mv_transformer_id"})',
        color="#f59e0b", unit="LPM")
    panels["ei_t_load"]    = label_panel("ei_t_load", "Load",
        'max(mv_transformer_load_kva{mv_transformer_id="$mv_transformer_id"})',
        color="#3b82f6", unit="kVA")
    panels["ei_t_oil_lvl"] = label_panel("ei_t_oil_lvl", "Oil Level",
        'min(mv_transformer_oil_level_pct{mv_transformer_id="$mv_transformer_id"})',
        color="#10b981", unit="%")
    panels["ei_t_buch"]    = label_panel("ei_t_buch", "Buchholz",
        'max(mv_transformer_buchholz_alarm{mv_transformer_id="$mv_transformer_id"})',
        color="#ef4444")

    # Transformer trend row (3)
    panels["ei_winding_all"] = ts_panel("ei_winding_all", "Winding Temp by Transformer + Winding (°C)",
        'max by (mv_transformer_id, winding)(mv_transformer_winding_temp_c)')
    panels["ei_oil_all"]     = ts_panel("ei_oil_all", "Oil Temp by Transformer (°C)",
        'max by (mv_transformer_id)(mv_transformer_oil_temp_c)')
    panels["ei_flow_all"]    = ts_panel("ei_flow_all", "Cooling Oil Flow by Transformer (LPM)",
        'min by (mv_transformer_id)(mv_transformer_cooling_oil_flow_lpm)')

    # Transformer load + radiator + level (3)
    panels["ei_load_all"]     = ts_panel("ei_load_all", "Load by Transformer (kVA)",
        'max by (mv_transformer_id)(mv_transformer_load_kva)')
    panels["ei_radiator_all"] = ts_panel("ei_radiator_all", "Radiator Temp by Transformer (°C)",
        'max by (mv_transformer_id)(mv_transformer_radiator_temp_c)')
    panels["ei_oil_lvl_all"]  = ts_panel("ei_oil_lvl_all", "Oil Level by Transformer (%)",
        'min by (mv_transformer_id)(mv_transformer_oil_level_pct)')

    # Compound ambient (1, wide) — the shared-infra signal
    panels["ei_compound_amb"] = ts_panel("ei_compound_amb",
        "MV Compound Outdoor Ambient Temp (°C) — siblings share this thermal envelope",
        'max by (mv_compound_id)(mv_compound_ambient_temp_c)')

    # Substation row (3)
    panels["ei_sub_mw"]  = ts_panel("ei_sub_mw", "Substation Export (MW)",
        'max(substation_export_power_mw)')
    panels["ei_sub_kv"]  = ts_panel("ei_sub_kv", "Busbar Voltage (kV)",
        'avg(substation_busbar_voltage_kv)')
    panels["ei_sub_hz"]  = ts_panel("ei_sub_hz", "Grid Frequency (Hz)",
        'avg(substation_grid_frequency_hz)')

    # Selected compound — siblings on that bay
    panels["ei_compound_winding"] = ts_panel("ei_compound_winding",
        "Sibling Transformer Winding Temps in Selected Compound (°C)",
        'max by (mv_transformer_id, winding)(mv_transformer_winding_temp_c{mv_compound_id="$mv_compound_id"})')

    panels["ei_compound_load"] = ts_panel("ei_compound_load",
        "Sibling Transformer Loads in Selected Compound (kVA)",
        'max by (mv_transformer_id)(mv_transformer_load_kva{mv_compound_id="$mv_compound_id"})')

    sections = [
        {"title": "Inventory & Grid", "cells": row_at(0,
            ("ei_trafos", 6, 3), ("ei_compounds", 6, 3),
            ("ei_export", 6, 3), ("ei_hz", 6, 3))},
        {"title": "Selected Transformer — Snapshot", "cells": row_at(0,
            ("ei_t_winding", 4, 3), ("ei_t_oil", 4, 3), ("ei_t_flow", 4, 3),
            ("ei_t_load", 4, 3), ("ei_t_oil_lvl", 4, 3), ("ei_t_buch", 4, 3))},
        {"title": "Transformer Thermal & Cooling", "cells": row_at(0,
            ("ei_winding_all", 8, 7), ("ei_oil_all", 8, 7), ("ei_flow_all", 8, 7))},
        {"title": "Transformer Load, Radiator, Oil Level", "cells": row_at(0,
            ("ei_load_all", 8, 6), ("ei_radiator_all", 8, 6), ("ei_oil_lvl_all", 8, 6))},
        {"title": "MV Compound — Shared Outdoor Ambient", "cells": row_at(0,
            ("ei_compound_amb", 24, 6))},
        {"title": "Substation Export Bus", "cells": row_at(0,
            ("ei_sub_mw", 8, 6), ("ei_sub_kv", 8, 6), ("ei_sub_hz", 8, 6))},
        {"title": "Selected Compound — Sibling Transformers (Blast Radius)",
         "cells": row_at(0,
            ("ei_compound_winding", 12, 6), ("ei_compound_load", 12, 6))},
    ]
    return {
        "name": "Adani — Electrical Infrastructure",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_TRANSFORMER, VAR_COMPOUND],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 4. Correlation & Blast Radius — spec §24.5
# ============================================================

def dashboard_correlation():
    """Cross-layer correlation workbench — variable-driven so the page
    reads as a generic chain (PPA → block → inverter → MV transformer →
    compound) for any selection, not the specific incident."""
    panels = {}

    # Selected-entity KPI strip (4)
    panels["cor_burn"]      = label_panel("cor_burn", "PPA Burn",
        'max(ppa_burn_rate{offtaker_id="$offtaker_id"})', color="#ef4444", unit="x")
    panels["cor_block_pr"]  = label_panel("cor_block_pr", "Block PR",
        'avg(block_performance_ratio{block_id="$block_id"}) * 100',
        color="#f59e0b", unit="%")
    panels["cor_inv_pwr"]   = label_panel("cor_inv_pwr", "Selected Inverter AC Power",
        'max(inverter_ac_power_kw{inverter_id="$inverter_id"})',
        color="#3b82f6", unit="kW")
    panels["cor_trafo_w"]   = label_panel("cor_trafo_w", "MV Trafo Winding (HV)",
        'max(mv_transformer_winding_temp_c{mv_transformer_id="$mv_transformer_id", winding="HV"})',
        color="#ef4444", unit="°C")

    # The chain — one axis. PPA deviation, block PR, inverter station
    # power, MV transformer winding temp. Reads top-to-bottom as effect →
    # cause if you scan left-to-right in time.
    panels["cor_chain"] = ts_panel("cor_chain",
        "Cause-to-Symptom Chain — MV winding ↑ → station kW ↓ → block PR ↓ → PPA deviation ↑",
        [
            'max(mv_transformer_winding_temp_c{mv_transformer_id="$mv_transformer_id", winding="HV"})',
            'max by (inverter_station_id)(inverter_station_ac_power_kw{block_id="$block_id"})',
            'avg(block_performance_ratio{block_id="$block_id"}) * 100',
            'max(ppa_dispatch_deviation_pct{offtaker_id="$offtaker_id"})',
        ])

    # Transformer detail row (3) — root-cause evidence
    panels["cor_trafo_flow"] = ts_panel("cor_trafo_flow",
        "MV Trafo Cooling Oil Flow (LPM) — drops before winding climbs",
        'min(mv_transformer_cooling_oil_flow_lpm{mv_transformer_id="$mv_transformer_id"})')
    panels["cor_trafo_oil"]  = ts_panel("cor_trafo_oil",
        "MV Trafo Oil & Radiator Temp (°C)",
        ['max(mv_transformer_oil_temp_c{mv_transformer_id="$mv_transformer_id"})',
         'max(mv_transformer_radiator_temp_c{mv_transformer_id="$mv_transformer_id"})'])
    panels["cor_trafo_load"] = ts_panel("cor_trafo_load",
        "MV Trafo Load (kVA) — throttles when cooling fails",
        'max(mv_transformer_load_kva{mv_transformer_id="$mv_transformer_id"})')

    # Block detail (3)
    panels["cor_block_mw"]  = ts_panel("cor_block_mw", "Block AC Power (MW)",
        'max(block_ac_power_mw{block_id="$block_id"})')
    panels["cor_block_avail"] = ts_panel("cor_block_avail", "Block Availability",
        'min(block_availability_ratio{block_id="$block_id"})')
    panels["cor_inv_temps"] = ts_panel("cor_inv_temps",
        "Inverters in Block — IGBT Temp (°C)",
        'max by (inverter_id)(inverter_igbt_temp_c{block_id="$block_id"})')

    # Inverter detail row (3)
    panels["cor_inv_eff"]    = ts_panel("cor_inv_eff", "Inverter Efficiency in Block (%)",
        'min by (inverter_id)(inverter_efficiency_pct{block_id="$block_id"})')
    panels["cor_inv_derate"] = ts_panel("cor_inv_derate", "Inverter Derate State",
        'max by (inverter_id)(inverter_derate_state{block_id="$block_id"})',
        variant="stacked-bar")
    panels["cor_inv_mppt"]   = ts_panel("cor_inv_mppt", "MPPT V × I",
        ['avg by (inverter_id)(inverter_mppt_voltage_v{block_id="$block_id"})',
         'avg by (inverter_id)(inverter_mppt_current_a{block_id="$block_id"})'])

    # Met / weather context (2)
    panels["cor_irr"]      = ts_panel("cor_irr",
        "POA Irradiance by Block (W/m²) — rules out weather/soiling as cause",
        'avg by (block_id)(met_irradiance_poa_w_m2)')
    panels["cor_soil"]     = ts_panel("cor_soil",
        "Soiling Loss by Block (%) — rules out a dust-storm story",
        'max by (block_id)(met_soiling_loss_pct)')

    # === Blast radius — same shared infra, every block / every offtaker ===

    # Same MV compound — sibling transformers
    panels["cor_blast_compound"] = ts_panel("cor_blast_compound",
        "BLAST — Sibling Transformers in Selected Compound, Winding Temp",
        'max by (mv_transformer_id, winding)(mv_transformer_winding_temp_c{mv_compound_id="$mv_compound_id"})')

    # Same MV transformer — every block + station downstream
    panels["cor_blast_trafo_block"] = ts_panel("cor_blast_trafo_block",
        "BLAST — Blocks fed by Selected MV Transformer, AC Power (MW)",
        'max by (block_id)(block_ac_power_mw{mv_transformer_id="$mv_transformer_id"})',
        variant="stacked-area")

    panels["cor_blast_trafo_pcs"] = ts_panel("cor_blast_trafo_pcs",
        "BLAST — PCS on Selected MV Transformer, AC Power (kW)",
        'max by (inverter_station_id)(inverter_station_ac_power_kw{mv_transformer_id="$mv_transformer_id"})')

    # Offtaker-level — every offtaker, dispatch deviation
    panels["cor_blast_offtaker"] = ts_panel("cor_blast_offtaker",
        "BLAST — Dispatch Deviation by Offtaker (%) — at-risk PPAs surface here",
        'max by (offtaker_id)(ppa_dispatch_deviation_pct)')

    # Fleet context — PR by block, top derated inverters
    panels["cor_fleet_pr"]  = ts_panel("cor_fleet_pr",
        "Fleet — Performance Ratio by Block",
        'avg by (block_id)(block_performance_ratio)')

    panels["cor_top_derate"] = ts_panel("cor_top_derate",
        "Fleet — Inverter Derate State (sum)",
        'sum by (inverter_id)(inverter_derate_state)', variant="stacked-bar")

    # Active alerts table
    panels["cor_alerts"] = ts_panel("cor_alerts", "Active Alerts",
        'max by (alert_name, alert_severity, suspected_layer, affected_entity_id)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "Selected Entity — KPIs", "cells": row_at(0,
            ("cor_burn", 6, 3), ("cor_block_pr", 6, 3),
            ("cor_inv_pwr", 6, 3), ("cor_trafo_w", 6, 3))},
        {"title": "Cause-to-Symptom Chain (one axis, four layers)",
         "cells": row_at(0, ("cor_chain", 24, 8))},
        {"title": "Root-Cause Evidence — MV Transformer", "cells": row_at(0,
            ("cor_trafo_flow", 8, 6), ("cor_trafo_oil", 8, 6), ("cor_trafo_load", 8, 6))},
        {"title": "Block Symptoms", "cells": row_at(0,
            ("cor_block_mw", 8, 6), ("cor_block_avail", 8, 6), ("cor_inv_temps", 8, 6))},
        {"title": "Inverter Inside the Block", "cells": row_at(0,
            ("cor_inv_eff", 8, 6), ("cor_inv_derate", 8, 6), ("cor_inv_mppt", 8, 6))},
        {"title": "Weather Sanity-Check (not weather, not soiling)",
         "cells": row_at(0, ("cor_irr", 12, 6), ("cor_soil", 12, 6))},
        {"title": "Blast Radius — Same MV Compound (sibling transformers)",
         "cells": row_at(0, ("cor_blast_compound", 24, 6))},
        {"title": "Blast Radius — Downstream of Selected MV Transformer",
         "cells": row_at(0,
            ("cor_blast_trafo_block", 12, 7), ("cor_blast_trafo_pcs", 12, 7))},
        {"title": "Blast Radius — PPA Deviation Across All Offtakers",
         "cells": row_at(0, ("cor_blast_offtaker", 24, 6))},
        {"title": "Fleet Context", "cells": row_at(0,
            ("cor_fleet_pr", 12, 6), ("cor_top_derate", 12, 6))},
        {"title": "Active Alerts", "cells": row_at(0,
            ("cor_alerts", 24, 6))},
    ]
    return {
        "name": "Adani — Correlation & Blast Radius",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_OFFTAKER, VAR_BLOCK, VAR_STATION, VAR_INVERTER,
                          VAR_TRANSFORMER, VAR_COMPOUND],
            "panels": panels,
            "sections": sections,
        },
    }

# ---------- main ----------

def main():
    """Emit SQL on stdout — pipe to psql."""
    builders = [
        dashboard_plant_health,
        dashboard_block_detail,
        dashboard_electrical_infra,
        dashboard_correlation,
    ]
    print("BEGIN;")
    for b in builders:
        d = b()
        spec_json = json.dumps(d["spec"]).replace("'", "''")
        name = d["name"].replace("'", "''")
        # Upsert by (org_id, name) — delete-then-insert is safest given the
        # name isn't a unique index.
        print(f"DELETE FROM maestro_dashboards WHERE org_id = '{ORG_ID}' AND name = '{name}' AND deleted_at IS NULL;")
        print(f"INSERT INTO maestro_dashboards (org_id, name, spec) VALUES ('{ORG_ID}', '{name}', '{spec_json}'::jsonb) RETURNING id, name;")
    print("COMMIT;")


if __name__ == "__main__":
    main()
