# Demo-video-script bootstrap prompt

Paste the block below into a fresh Claude Code session (after `/clear`)
to get back a recording-ready video script for the Airtel demo.

---

```
You are writing a 5–7 minute demo video script for Cardinal sales. The
demo shows Cardinal correlating a managed-PostgreSQL SLO breach down
through VMware infrastructure for an Airtel Cloud Managed Services
audience.

## Environment (already live, do not deploy or change anything)

- Cluster: prod-us-east-2-global (EKS account 992382549843)
- Simulator pod: namespace `airtel-demo`, deploy `dbaas`, fault HTTP on
  pod port 9999. Reach via `kubectl -n airtel-demo port-forward
  svc/dbaas-faults 9999:9999`.
- Telemetry lands in Cardinal HQ org (id
  `c4375e34-dfcf-498a-8ba3-a02d119baf82`). The narrator opens the UI
  there.

## Four dashboards (in the Cardinal HQ org)

1. **Airtel — Tenant Health Overview** — fleet posture: tenant count,
   breaching count, at-risk count, fleet SLO trend, tier and AZ
   rollups, app latency, fleet workload, shared infra.
2. **Airtel — Tenant PostgreSQL Detail** — per-DB ops view, variables
   `$pg_instance` + `$vm_name`. Latency analysis (p95 by query_class,
   p99 from histogram, bucket fan-out), throughput, sessions by wait
   event, checkpoint + WAL, locks/cache/replication, VM correlation row.
3. **Airtel — VMware Infrastructure** — datacenter ops: inventory,
   datastore perf, capacity, host CPU/memory/network, VM CPU/memory/disk,
   Linux guest pressure. Variable `$datastore_id` drives the blast-
   radius helper.
4. **Airtel — Correlation & Blast Radius** — variable-driven workbench.
   Dropdowns: tenant, pg_instance, vm_name, esxi_host_id, datastore_id,
   storage_array. Selected-entity KPIs, latency chain
   (datastore → VM disk → DB p95), host + guest evidence, DB-internal
   evidence (WAL, wait events), application symptoms, three blast-
   radius cuts (same datastore / same host / same storage array),
   fleet context, active alerts.

## Failure profile to drive

`datastore_latency_shared_infra` — the canonical demo. Activates with:
`curl -sX POST 'localhost:9999/faults/activate?profile=datastore_latency_shared_infra'`
35-min run, 2-min ramp up, 5-min ramp down. Onset timeline (per
spec §28.2):
- +0m datastore-202 (ds-gold-delhi-02) latency + queue depth spike
- +2m host-1017 (esx-delhi-a-17) disk + CPU
- +3m vm-bajaj-pg-01 VMware disk + CPU ready
- +4m vm-bajaj-pg-01 Linux iowait + disk_io_now + CPU steal
- +7m pg-bajaj-01 query p95 + checkpoint pressure + cache hit drop
- +10m tenant_bajaj_finance probe latency/success degraded
- +11m tenant_bajaj_finance SLO burn fires
- +12m vm-indigo-pg-01 disk/CPU at AT-RISK amplitudes
- +15m tenant_indigo_ops SLO at at-risk amplitude

After ~12 min in plateau, every dashboard tells the same story. The
defaults on the Correlation workbench are pre-set to the canonical
entities (Bajaj / pg-bajaj-01 / vm-bajaj-pg-01 / host-1017 /
datastore-202 / san-delhi-gold-01) so the narrator doesn't have to
type anything.

## Demo narrative arc (your script should follow this)

1. Cold open on **Tenant Health Overview**. Operator's morning view.
   Point out: 6 tenants, 4 DB instances, fleet posture. One tenant
   (Bajaj Finance) breaching SLO burn rate, IndiGo at-risk. Probe
   success degraded for one AZ.
2. Drill from the breaching tenant into **Tenant PostgreSQL Detail**.
   Pick pg-bajaj-01. p95 spike, p99 from histogram, IO wait events
   dominating, checkpoint sync rate climbing.
3. Question: is this Postgres, or something underneath? Show the VM
   correlation row at the bottom — same VM iowait + VMware disk write
   latency are spiking too.
4. Cross to **VMware Infrastructure**. Datastore write latency on
   ds-gold-delhi-02 is elevated, queue depth saturated. ESXi host
   esx-delhi-a-17 disk latency elevated. Other hosts/datastores quiet.
5. Open **Correlation & Blast Radius**. Show the latency chain panel
   — datastore, VM, DB plotted together, ramping in sequence. Then
   the three blast-radius cuts: same datastore (Bajaj + IndiGo VMs),
   same host (any other tenant VMs there), same storage array.
6. Punchline: Cardinal followed the attributes through the stack —
   pg_instance → vm_uuid → datastore_id → other tenants — and
   identified that **vm-indigo-pg-01 (IndiGo Operations)** is the
   next tenant likely to breach. No hard-coded dashboards, no
   pre-built correlation rules — just attribute joins on real telemetry.

## What to output

A scene-by-scene script with:
- **Timestamps** (e.g. 00:00 – 00:25) targeting 5–7 minutes total
- **On-screen action** ("port-forward, activate profile", "click into
  pg-bajaj-01", "scroll to VM Correlation row")
- **Narrator voice-over** in plain conversational prose (Airtel ops
  audience, no Cardinal-internal jargon, no Lakerunner mentions)
- **Lower-third callouts** for key metric names or values that should
  flash on-screen
- A short **pre-roll preparation list** so the recorder activates the
  profile and waits the right amount of time before hitting record
  (the SLO burn doesn't fire for ~11 min — script around that)

Keep the script tight, recorder-friendly, and free of any reference to
Bajaj/IndiGo being synthetic. Treat them as real Airtel customers.
```

---

That's the prompt. Save it, `/clear`, paste it, and the next session
will generate the script without needing any other context.
