# Adani Demo — Video Script

A walkthrough of the four Adani Khavda solar farm dashboards, showing
how a Power Purchase Agreement (PPA) dispatch deviation gets traced
down to a shared MV transformer compound — and how the same
dashboards reveal which other offtaker is about to be hit.

- **Audience:** Renewable-ops engineers and platform SREs. Know solar
  block / inverter / PPA at the basic level. Have probably never used
  Cardinal before.
- **Goal:** Convince them, with grounded specifics, that these
  dashboards actually surface MV-transformer → block → PPA causality.
  No magic. No hand-waving.
- **Runtime:** ~6:00 (5–7 min spec)
- **Tone:** Conversational. Each dashboard answers one question. The
  story is the teaching device — they should be able to repeat the
  walkthrough themselves after watching.
- **Customer treatment:** SECI Phase III, GUVNL, and Adani Electricity
  Mumbai are real Adani Green Energy offtakers throughout. Block
  / inverter / transformer IDs are Khavda-realistic.

---

## Pre-roll prep (BEFORE recording)

```bash
# Terminal 1 — port-forward the simulator
kubectl -n adani-demo port-forward svc/solar-faults 9999:9999

# Terminal 2 — activate the canonical profile
curl -sX POST 'localhost:9999/faults/activate?profile=mv_transformer_winding_overheat'
```

Wait **13 minutes** before recording. The SECI dispatch deviation
crosses the breach threshold at ~+11m and the sibling transformer
T-04-B doesn't enter the at-risk band until ~+13m. Earlier than that
and the blast-radius story isn't on the screen yet.

Open four browser tabs (Cardinal HQ org), each with **Last 30 minutes**
+ auto-refresh on:

1. **Adani — Plant Health Overview** (`Offtaker = seci_phase_iii`)
2. **Adani — Block Detail** (`Block = block-04`, `Station = IS-04-01`,
   `Inverter = INV-04-01-01`, `Tracker = TRK-12`, `Met = MET-04-1`)
3. **Adani — Electrical Infrastructure** (`MV Transformer = T-04-A`,
   `MV Compound = mvc-04`)
4. **Adani — Correlation & Blast Radius** (defaults pre-fill SECI /
   block-04 / IS-04-01 / INV-04-01-01 / T-04-A / mvc-04)

The profile runs 35 minutes with a 5-minute ramp-down, so you have
about 15 minutes of comfortable plateau to record in.

---

## How to think about the four dashboards

Before we start, a one-liner per dashboard so the walkthrough has shape:

1. **Plant Health Overview** — *"Is any offtaker missing their dispatch right now?"*
2. **Block Detail** — *"What is this one block doing?"*
3. **Electrical Infrastructure** — *"What does the substation say about the iron underneath?"*
4. **Correlation & Blast Radius** — *"Put it all on one axis, and tell me who else is exposed."*

Each scene below answers one of those questions.

---

## SCENE 1 — Is anyone missing dispatch?
**Timecode:** 00:00 – 01:00
**Dashboard:** Adani — Plant Health Overview

**On-screen action:** Page already open. Slow pointer across the
**Fleet Posture** tiles, then **Selected Offtaker — PPA Snapshot**,
then **Fleet PPA Trends**.

**Narrator (VO):**
> This is the page our renewable-ops engineers open every morning. Every Khavda offtaker — every PPA — on one dashboard. The question this page answers is the simplest one: *is anyone missing their dispatch right now?*
>
> The *Fleet Posture* row at the top is the morning headline. Three offtakers. Six fifty-megawatt blocks. Ninety-six inverters. One offtaker breaching PPA.
>
> A quick word on what "PPA" means here so the rest of the demo lands. Every offtaker has a day-ahead schedule — how many megawatts they expect us to dispatch each fifteen-minute block. SECI's commitment for Phase III is eighty-five megawatts during this slot. GUVNL is one twenty-eight. Adani Mumbai forty-two. We watch two things to know whether we're meeting the schedule.
>
> One — what the substation actually exports. That's *Substation Export* in the *Plant Production* row, off the 220 kV pooling bus.
>
> Two — what each offtaker actually receives, attributable through the meter contract. That feeds *Dispatch Deviation*.
>
> *PPA Burn Rate* tells us how fast we're spending the offtaker's monthly compliance buffer. Anything above one and we're losing buffer faster than we earn it back; above ten and we're triggering the Deviation Settlement Mechanism charge schedule.

**On-screen action:** Pause on *PPA Burn Rate* and *Dispatch Deviation*
tiles for SECI, then move to **Fleet PPA Trends** and **Block
Production**.

**Narrator (VO):**
> Now look at SECI Phase III. Burn rate at fourteen. Dispatch deviation at eighteen percent against a tolerance band of five. *Dispatch Deviation by Offtaker* — SECI's line is the one lifted off the floor. GUVNL and Adani Mumbai are flat at baseline.
>
> *Performance Ratio by Block* — two blocks have dropped together: block-04 and block-12. Their PR is in the seventy percent band. The other four blocks are at ninety-five plus, where they should be. *AC Power by Block* tells the same story stacked — those two blocks are short fifteen megawatts each.
>
> Two blocks together is a clue. If it were one block, we'd be looking at one inverter station. Two blocks means whatever is wrong is *upstream of the blocks*. Let's drill in.

**Lower-thirds:**
- `PPA: dispatch deviation tolerance 5% gold / 7% silver`
- `Two signals: substation meter + per-offtaker schedule attribution`
- `SECI Phase III · Burn 14× · Deviation 18%`
- `Two blocks down together → upstream of the block layer`

---

## SCENE 2 — What is this block doing?
**Timecode:** 01:00 – 02:20
**Dashboard:** Adani — Block Detail

**On-screen action:** Switch tab. Variables at `block-04` /
`IS-04-01` / `INV-04-01-01` / `TRK-12` / `MET-04-1`. Pause on the
**Block Posture** tile strip. Then to **Inverter Stations (PCS)**,
then **Inverter Production & Thermal**, then **Inverter Cooling &
Derate**.

**Narrator (VO):**
> Second dashboard. This one zooms in on a single block — block-04, a fifty-megawatt unit. The question now is: *what is this block doing?*
>
> The *Block Posture* strip up top is the snapshot. AC power at twenty-three megawatts against a fifty-megawatt nameplate — we're missing more than half of it. PR at seventy-four percent — normally ninety-eight. Availability still ninety-five, so the inverters aren't tripped, they're just producing less.
>
> *Inverter Stations* — four PCS units in the block. All four are derating in lockstep. AC power per station is way down. AC voltage is dipping below six hundred volts. Every PCS is doing the same thing at the same time, which already tells us this isn't a single inverter fault — it's the bus they all feed.
>
> *Inverter Production & Thermal* — sixteen inverters in the block, every one of them is producing about half what it should. *Inverter IGBT Temp* — every inverter is in the high eighties to low hundreds. Hot, but not catastrophic. *Inverter Cooling Fan RPM* — fans are flat. So the inverters aren't slowing themselves down on internal temp; something external is asking them to derate.

**On-screen action:** Move to **Trackers** and **Met Station**.

**Narrator (VO):**
> *Trackers* and *Met Station* are the sanity check. *Tracker Angle vs Target* — TRK-04 is tracking. The motor current is normal. *POA vs GHI* — irradiance is nine hundred watts per square meter, normal for this time of day. *Soiling Loss* — under two percent. No dust storm. The sun is fine. The trackers are fine. The modules are clean.
>
> So we have sixteen inverters across four PCS units, all derating together, while the sun and the modules are fine. The inverters are healthy but being told to back off. That points at the AC side — what's downstream of these PCS units.

**Lower-thirds:**
- `Block-04 · 23 MW of 50 MW nameplate · PR 74%`
- `All 16 inverters derating in sync — not a single-unit fault`
- `POA 900 W/m² · soiling 1.5% — weather and modules ruled out`
- `Symptom is downstream of the inverter — look at the iron`

---

## SCENE 3 — What does the substation say?
**Timecode:** 02:20 – 03:40
**Dashboard:** Adani — Electrical Infrastructure

**On-screen action:** Switch tab. Variables at `T-04-A` and `mvc-04`.
Pause on the **Selected Transformer — Snapshot** tile strip. Then to
**Transformer Thermal & Cooling**, hover the legend on *Winding Temp by
Transformer + Winding* to highlight `T-04-A`. Same with *Cooling Oil
Flow*.

**Narrator (VO):**
> Third dashboard. We've left the inverter view entirely. This is the substation engineer's view: every MV transformer feeding the 220 kV pooling bus, plus the outdoor compounds they live in. Same time window. The question is: *what does the substation say is happening underneath?*
>
> *Selected Transformer Snapshot* — T-04-A. Winding HV temperature at one twenty-eight degrees Celsius. The Siemens GEAFOL spec sheet calls one ten the warning band; one forty trips the bay. We are sitting in the warning band and climbing. Oil temperature at ninety-five. Cooling oil flow at twenty-five liters per minute against a normal eighty. That's the smoking gun.
>
> *Cooling Oil Flow by Transformer* — every transformer except T-04-A is at seventy-five to eighty-five LPM. T-04-A is at twenty. Either the MOV is degraded, the radiator is clogged, or the pump is cavitating. Whatever it is, the transformer can't shed heat — and the protection relay is throttling it to keep the windings from cooking.
>
> *Load by Transformer* corroborates. T-04-A's load has dropped from sixty MVA down into the thirties because the bay is throttling it. Lower load means less heat input — but at the cost of less power on the secondary side, which is exactly what the inverters were complaining about in scene two.

**On-screen action:** Scroll to **MV Compound — Shared Outdoor Ambient**
and **Selected Compound — Sibling Transformers (Blast Radius)**.
`mv_compound_id` variable is already `mvc-04`.

**Narrator (VO):**
> And here's where the demo gets interesting. *MV Compound Ambient Temp* — every compound at Khavda has a number. `mvc-04` is at fifty-three degrees Celsius. The other two compounds are at thirty-six and thirty-eight. Forty Indian summer degrees is normal in Kutch; fifty-three is the radiator bank itself dumping waste heat back into the air the *next* transformer breathes.
>
> *Sibling Transformer Winding Temps in Selected Compound* — T-04-B shares this compound with T-04-A. Different transformer, different secondary bus, different block — but the same outdoor air. T-04-B's winding temp is at ninety-five degrees and climbing. Still inside its safe band, but climbing.
>
> Shared cooling doesn't care about contract boundaries. If T-04-B is in this compound, the offtaker fed by T-04-B is exposed.

**Lower-thirds:**
- `T-04-A · winding HV 128°C · oil flow 25 LPM (normal 80)`
- `mvc-04 ambient 53°C (other compounds 36–38°C)`
- `T-04-B sharing compound — winding 95°C, climbing`
- `Shared cooling → shared fate`

---

## SCENE 4 — One axis, the whole story, and who else is exposed
**Timecode:** 03:40 – 05:20
**Dashboard:** Adani — Correlation & Blast Radius

**On-screen action:** Switch tab. The six variables at the top are
pre-filled. Pause on **Selected Entity — KPIs**, then expand the
**Cause-to-Symptom Chain** panel.

**Narrator (VO):**
> Fourth and last dashboard. The first three each answered one question — what's breaching, what's the block doing, what does the substation say. This one puts the whole story on a single axis, and then tells us who else is going to be on the call.
>
> The headline panel — *Cause-to-Symptom Chain: MV winding up, station kW down, block PR down, PPA deviation up*. Four layers of the stack, one time axis.
>
> The MV winding temp line rises first. Then the station AC power line drops. Then the block PR drops. Then the PPA dispatch deviation rises. The cause is at the bottom of the stack and it lifts off the floor *before* the symptom at the top. If you ever wanted a visual proof that the MV transformer is causing the PPA breach — this panel is it.

**On-screen action:** Scroll to **Root-Cause Evidence — MV Transformer**,
then **Block Symptoms**, then **Inverter Inside the Block**.

**Narrator (VO):**
> The rows underneath are the corroborating evidence. *Root-Cause Evidence* — the cooling oil flow drops first, then the oil and radiator temps rise, then the load throttles. That sequence is exactly what a Buchholz-relay write-up looks like in the post-incident report.
>
> *Block Symptoms* — block AC power and availability drop together. *Inverters in Block — IGBT Temp* climbs. *Efficiency* drops. *Derate State* goes from zero to one across the entire block.
>
> And the *Weather Sanity-Check* row — POA irradiance by block, soiling loss by block — is flat across every block. So whatever this is, it isn't a dust storm and it isn't an early-morning irradiance ramp. That row exists specifically to head off the "could it just be weather?" question that always comes up in the post-mortem.

**On-screen action:** Scroll through the **Blast Radius** sections in
sequence: *Same MV Compound*, *Downstream of Selected MV Transformer*,
*PPA Deviation Across All Offtakers*.

**Narrator (VO):**
> Now the part this dashboard was built for. Three blast-radius cuts.
>
> *Blast Radius — Sibling Transformers in Selected Compound*: T-04-A and T-04-B are both in `mvc-04`. T-04-B's winding is climbing toward the warning band, sharing the same outdoor air.
>
> *Blast Radius — Downstream of Selected MV Transformer*: which blocks feed through T-04-A. Block-04 and block-12. Both fed by the same iron. If T-04-A throttles further or trips, both blocks go to zero. The PCS sub-panel breaks it down further — every PCS unit on both blocks.
>
> *Blast Radius — Dispatch Deviation by Offtaker*: SECI is the obvious breach. GUVNL — the offtaker that owns block-06 and block-14, both fed by T-04-B — is starting to lift. Their deviation is at four percent. Their PPA tolerance is seven, so they're inside the band, but the trajectory is the wrong direction.
>
> SECI is the offtaker we're calling first. GUVNL is the second phone call we make before they make it to us.

**Lower-thirds:**
- `Cause-to-symptom chain: MV winding leads PPA deviation by ~8 minutes`
- `Sibling on mvc-04: T-04-B → blocks 06 and 14 → GUVNL`
- `Active alert: PPA dispatch deviation breach · severity critical`
- `Predicted next breach: guvnl_state`

---

## SCENE 5 — Recap and the point
**Timecode:** 05:20 – 06:00
**Dashboard:** Adani — Correlation & Blast Radius (still)

**On-screen action:** Zoom out. Camera holds on the workbench page as
a whole.

**Narrator (VO):**
> So that's the walkthrough.
>
> Dashboard one told us an offtaker was missing dispatch. Dashboard two told us two blocks were derated together. Dashboard three told us a shared MV transformer was cooking, and a sibling transformer in the same compound was at-risk. Dashboard four put it on one axis and named the next offtaker that's going to feel it.
>
> Two things to take away. First — none of this required manual investigation or vendor SCADA forensics. The dashboards already exist, the metrics already flow, and the path from PPA dispatch down to shared MV iron is something a renewable-ops engineer can walk through in a few minutes.
>
> Second — when MV infrastructure is shared, the offtaker that breaches first is just the loudest one. The blast-radius panels exist so the next breach is a phone call we make to GUVNL, not a phone call we get from them.
>
> That's the demo. Thanks for watching.

**Lower-thirds:**
- `4 dashboards · 4 questions · 1 story`
- `PPA → Block → Inverter → MV Transformer → Compound → Blast Radius`
- `Get ahead of the next breach, don't react to it`

---

## Recorder notes

- **Pacing:** Don't rush the *MV Compound Ambient Temp* panel in scene
  3 — the "fifty-three degrees in one compound, thirty-six in the
  others" reveal is the moment the audience realises this isn't a
  per-transformer problem. Let it sit on screen.
- **Mouse discipline:** Slow pointer, hover legends rather than click.
  Highlighting one series in a busy panel (e.g., `T-04-A` on *Winding
  Temp by Transformer*) is how the audience learns to read these
  dashboards.
- **Frame each scene with its question** — say the question out loud
  before pointing at panels. That's the teaching scaffold.
- **If you re-record:** Re-activate the profile fresh. Running it
  twice back-to-back stacks amplitudes and ruins the calm baseline on
  the unaffected transformers and compounds in scene 3.
- **Backup take:** If *Sibling Transformer Winding Temps* doesn't yet
  show T-04-B lifting, you're still inside the 13-minute window — wait.
  Scene 4's punchline depends on a visible T-04-B line.
- **Do not say** SECI Phase III, GUVNL, or Adani Electricity Mumbai
  data is synthetic. They are real Adani Green Energy offtakers for
  the duration of this recording.
