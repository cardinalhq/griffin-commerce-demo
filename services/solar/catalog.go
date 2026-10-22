// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

// catalog.go is the static entity catalog from the Adani solar farm
// simulator spec (docs/specs/adani-solar-farm-simulator.md §4). Every
// metric and log walks these pointers so the correlation join keys
// (offtaker_id, block_id, inverter_station_id, inverter_id,
// mv_transformer_id, mv_compound_id) line up across all signals.

const (
	siteID    = "khavda-1"
	plantName = "Khavda Renewable Energy Park"
	region    = "in-west-kutch-1"
	stateName = "gujarat"
	country   = "india"

	substationID = "gss-khavda-01"

	mvCompound04 = "mvc-04"
	mvCompound08 = "mvc-08"
	mvCompound10 = "mvc-10"

	mvTrafoT04A = "T-04-A"
	mvTrafoT04B = "T-04-B"
	mvTrafoT08  = "T-08"
	mvTrafoT10  = "T-10"

	offtakerSECI   = "seci_phase_iii"
	offtakerGUVNL  = "guvnl_state"
	offtakerAdaniM = "adani_mumbai"

	// Incident roles — non-empty for entities the §29 profiles target.
	rolePrimaryTransformer = "primary_impacted_transformer"
	roleAtRiskTransformer  = "at_risk_transformer"
	rolePrimaryBlock       = "primary_impacted_block"
	roleAtRiskBlock        = "at_risk_block"
	rolePrimaryOfftaker    = "primary_impacted_offtaker"
	roleAtRiskOfftaker     = "at_risk_offtaker"
	rolePrimaryInverter    = "primary_impacted_inverter"
	rolePrimaryTracker     = "primary_impacted_tracker"
	roleStringPID          = "string_pid_degraded"
)

type Offtaker struct {
	OfftakerID            string
	PPAID                 string
	Name                  string
	Tier                  string // platinum, gold, silver
	ContractedCapacityMW  float64
	ScheduledDispatchMW   float64 // day-ahead schedule mid-band
	DeviationToleranceMin float64 // tolerance band in %
	IncidentRole          string
	state                 *offtakerState
}

type MVCompound struct {
	ID           string
	OutdoorBay   string
	IncidentRole string
}

type MVTransformer struct {
	ID            string
	Compound      *MVCompound
	RatedKVA      float64
	HVKv          float64
	LVKv          float64
	Vendor        string
	Model         string
	YearInstalled int
	IncidentRole  string
	state         *transformerState
}

type Block struct {
	ID                  string
	BlockName           string
	NameplateCapacityMW float64
	Offtaker            *Offtaker
	Transformer         *MVTransformer
	IncidentRole        string
	state               *blockState
}

type InverterStation struct {
	ID                 string
	Block              *Block
	NameplateKW        float64
	RatedAcVoltageV    float64
	state              *inverterStationState
}

type Inverter struct {
	ID           string
	Station      *InverterStation
	Vendor       string
	Model        string
	NameplateKW  float64
	StringCount  int
	IncidentRole string
	state        *inverterState
}

type Tracker struct {
	ID           string
	Block        *Block
	RowCount     int
	IncidentRole string
	state        *trackerState
}

type MetStation struct {
	ID           string
	Block        *Block
	state        *metState
}

type Substation struct {
	ID                  string
	BusbarKv            float64
	GridConnectionPoint string
	state               *substationState
}

type Catalog struct {
	Site          string
	Offtakers     []*Offtaker
	Substation    *Substation
	MVCompounds   []*MVCompound
	Transformers  []*MVTransformer
	Blocks        []*Block
	Stations      []*InverterStation
	Inverters     []*Inverter
	Trackers      []*Tracker
	MetStations   []*MetStation
}

// NewCatalog returns the spec §4 entity catalog with cross-links resolved
// and per-entity state allocated.
func NewCatalog() *Catalog {
	c := &Catalog{Site: siteID}

	offtakers := []*Offtaker{
		{OfftakerID: offtakerSECI, PPAID: "ppa-seci-phase-iii-2024",
			Name: "SECI Phase III", Tier: "platinum",
			ContractedCapacityMW: 100, ScheduledDispatchMW: 85,
			DeviationToleranceMin: 5,
			IncidentRole: rolePrimaryOfftaker},
		{OfftakerID: offtakerGUVNL, PPAID: "ppa-guvnl-2023",
			Name: "Gujarat Urja Vikas Nigam", Tier: "gold",
			ContractedCapacityMW: 150, ScheduledDispatchMW: 128,
			DeviationToleranceMin: 7,
			IncidentRole: roleAtRiskOfftaker},
		{OfftakerID: offtakerAdaniM, PPAID: "ppa-aeml-2024",
			Name: "Adani Electricity Mumbai", Tier: "gold",
			ContractedCapacityMW: 50, ScheduledDispatchMW: 42,
			DeviationToleranceMin: 7},
	}
	for _, o := range offtakers {
		o.state = newOfftakerState(o)
	}
	c.Offtakers = offtakers
	offtakerByID := map[string]*Offtaker{}
	for _, o := range offtakers {
		offtakerByID[o.OfftakerID] = o
	}

	c.Substation = &Substation{
		ID: substationID, BusbarKv: 220,
		GridConnectionPoint: "POI-Khavda-North-220kV",
	}
	c.Substation.state = newSubstationState(c.Substation)

	compounds := []*MVCompound{
		{ID: mvCompound04, OutdoorBay: "bay-north-04", IncidentRole: rolePrimaryTransformer},
		{ID: mvCompound08, OutdoorBay: "bay-north-08"},
		{ID: mvCompound10, OutdoorBay: "bay-south-10"},
	}
	c.MVCompounds = compounds
	compoundByID := map[string]*MVCompound{}
	for _, mc := range compounds {
		compoundByID[mc.ID] = mc
	}

	transformers := []*MVTransformer{
		{ID: mvTrafoT04A, Compound: compoundByID[mvCompound04], RatedKVA: 75000,
			HVKv: 33, LVKv: 0.6, Vendor: "Siemens", Model: "GEAFOL-75MVA",
			YearInstalled: 2024, IncidentRole: rolePrimaryTransformer},
		{ID: mvTrafoT04B, Compound: compoundByID[mvCompound04], RatedKVA: 75000,
			HVKv: 33, LVKv: 0.6, Vendor: "Siemens", Model: "GEAFOL-75MVA",
			YearInstalled: 2024, IncidentRole: roleAtRiskTransformer},
		{ID: mvTrafoT08, Compound: compoundByID[mvCompound08], RatedKVA: 75000,
			HVKv: 33, LVKv: 0.6, Vendor: "Siemens", Model: "GEAFOL-75MVA",
			YearInstalled: 2024},
		{ID: mvTrafoT10, Compound: compoundByID[mvCompound10], RatedKVA: 50000,
			HVKv: 33, LVKv: 0.6, Vendor: "CG Power", Model: "ECOSAVE-50MVA",
			YearInstalled: 2024},
	}
	for _, mt := range transformers {
		mt.state = newTransformerState(mt)
	}
	c.Transformers = transformers
	trafoByID := map[string]*MVTransformer{}
	for _, mt := range transformers {
		trafoByID[mt.ID] = mt
	}

	type blockSpec struct {
		ID, Name, OfftakerID, TransformerID, IncidentRole string
		NameplateMW                                       float64
	}
	blockSpecs := []blockSpec{
		{ID: "block-04", Name: "Block 04 — North Tilt", OfftakerID: offtakerSECI,
			TransformerID: mvTrafoT04A, NameplateMW: 50, IncidentRole: rolePrimaryBlock},
		{ID: "block-06", Name: "Block 06 — North Centre", OfftakerID: offtakerGUVNL,
			TransformerID: mvTrafoT04B, NameplateMW: 50, IncidentRole: roleAtRiskBlock},
		{ID: "block-08", Name: "Block 08 — North Wing", OfftakerID: offtakerGUVNL,
			TransformerID: mvTrafoT08, NameplateMW: 50},
		{ID: "block-10", Name: "Block 10 — South Wing", OfftakerID: offtakerAdaniM,
			TransformerID: mvTrafoT10, NameplateMW: 50},
		{ID: "block-12", Name: "Block 12 — North Edge", OfftakerID: offtakerSECI,
			TransformerID: mvTrafoT04A, NameplateMW: 50, IncidentRole: rolePrimaryBlock},
		{ID: "block-14", Name: "Block 14 — North Inner", OfftakerID: offtakerGUVNL,
			TransformerID: mvTrafoT04B, NameplateMW: 50, IncidentRole: roleAtRiskBlock},
	}
	for _, bs := range blockSpecs {
		b := &Block{
			ID: bs.ID, BlockName: bs.Name, NameplateCapacityMW: bs.NameplateMW,
			Offtaker: offtakerByID[bs.OfftakerID], Transformer: trafoByID[bs.TransformerID],
			IncidentRole: bs.IncidentRole,
		}
		b.state = newBlockState(b)
		c.Blocks = append(c.Blocks, b)
	}

	// 4 inverter stations per block × 4 inverters per station = 96 inverters
	// (~12.5 MW per station, 50 MW per block, 300 MW total).
	const stationsPerBlock = 4
	const invertersPerStation = 4
	const inverterKW = 3125.0

	for _, b := range c.Blocks {
		for s := 1; s <= stationsPerBlock; s++ {
			sid := stationID(b.ID, s)
			station := &InverterStation{
				ID: sid, Block: b,
				NameplateKW: inverterKW * float64(invertersPerStation),
				RatedAcVoltageV: 600,
			}
			station.state = newInverterStationState(station)
			c.Stations = append(c.Stations, station)
			for i := 1; i <= invertersPerStation; i++ {
				iid := inverterID(b.ID, s, i)
				incidentRole := ""
				// Inverter cooling fault profile targets INV-08-02-03.
				if iid == "INV-08-02-03" {
					incidentRole = rolePrimaryInverter
				}
				inv := &Inverter{
					ID: iid, Station: station,
					Vendor: "Sungrow", Model: "SG3125HV",
					NameplateKW: inverterKW, StringCount: 16,
					IncidentRole: incidentRole,
				}
				inv.state = newInverterState(inv)
				c.Inverters = append(c.Inverters, inv)
			}
		}
		// One tracker controller aggregate per block (24 rows / 480 modules each
		// for Khavda-scale blocks).
		trkID := "TRK-" + lastTwo(b.ID)
		trkRole := ""
		if b.ID == "block-12" {
			trkRole = rolePrimaryTracker
		}
		trk := &Tracker{ID: trkID, Block: b, RowCount: 24, IncidentRole: trkRole}
		trk.state = newTrackerState(trk)
		c.Trackers = append(c.Trackers, trk)
		// Two met stations per block (POA + GHI redundant pair).
		for m := 1; m <= 2; m++ {
			mid := "MET-" + lastTwo(b.ID) + "-" + itoa(m)
			ms := &MetStation{ID: mid, Block: b}
			ms.state = newMetStationState(ms)
			c.MetStations = append(c.MetStations, ms)
		}
	}

	return c
}

// stationID returns canonical "IS-NN-XX" given a block id ("block-04")
// and station number 1..N.
func stationID(blockID string, n int) string {
	return "IS-" + lastTwo(blockID) + "-" + zeroPad(n)
}

// inverterID returns canonical "INV-NN-SS-II" given block, station, inverter.
func inverterID(blockID string, station, inv int) string {
	return "INV-" + lastTwo(blockID) + "-" + zeroPad(station) + "-" + zeroPad(inv)
}

func lastTwo(blockID string) string {
	if len(blockID) < 2 {
		return blockID
	}
	return blockID[len(blockID)-2:]
}

func zeroPad(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [16]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// -- helpers --

// BlocksOnTransformer returns every block whose primary inverter step-up
// feeds through the given MV transformer. The mv_transformer_winding_overheat
// profile uses this to identify blocks under T-04-A.
func (c *Catalog) BlocksOnTransformer(trafoID string) []*Block {
	var out []*Block
	for _, b := range c.Blocks {
		if b.Transformer != nil && b.Transformer.ID == trafoID {
			out = append(out, b)
		}
	}
	return out
}

// TransformersInCompound returns every MV transformer sharing the given
// outdoor compound. The blast-radius story walks T-04-A → mvc-04 →
// {T-04-A, T-04-B} via this helper.
func (c *Catalog) TransformersInCompound(compoundID string) []*MVTransformer {
	var out []*MVTransformer
	for _, mt := range c.Transformers {
		if mt.Compound != nil && mt.Compound.ID == compoundID {
			out = append(out, mt)
		}
	}
	return out
}

// StationsInBlock returns the inverter stations in a block.
func (c *Catalog) StationsInBlock(blockID string) []*InverterStation {
	var out []*InverterStation
	for _, s := range c.Stations {
		if s.Block != nil && s.Block.ID == blockID {
			out = append(out, s)
		}
	}
	return out
}

// InvertersInBlock returns every inverter in the given block.
func (c *Catalog) InvertersInBlock(blockID string) []*Inverter {
	var out []*Inverter
	for _, inv := range c.Inverters {
		if inv.Station != nil && inv.Station.Block != nil && inv.Station.Block.ID == blockID {
			out = append(out, inv)
		}
	}
	return out
}

// InvertersOnTransformer returns every inverter feeding through the given
// MV transformer (i.e. every inverter whose block's transformer matches).
func (c *Catalog) InvertersOnTransformer(trafoID string) []*Inverter {
	var out []*Inverter
	for _, inv := range c.Inverters {
		if inv.Station != nil && inv.Station.Block != nil &&
			inv.Station.Block.Transformer != nil &&
			inv.Station.Block.Transformer.ID == trafoID {
			out = append(out, inv)
		}
	}
	return out
}

// BlocksForOfftaker returns every block under the given PPA.
func (c *Catalog) BlocksForOfftaker(offtakerID string) []*Block {
	var out []*Block
	for _, b := range c.Blocks {
		if b.Offtaker != nil && b.Offtaker.OfftakerID == offtakerID {
			out = append(out, b)
		}
	}
	return out
}
