// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Typed client for the fault-injection control plane. All requests go
// through the local proxy at /admin/faults*, which Vite (dev) and nginx
// (prod) forward to controlplane:8086.

export interface Knob {
  key: string;
  service: string;
  kind: string;
  probability?: number;
  latencyMs?: number;
  statusCode?: number;
  target?: string;
  startedAt: string; // RFC3339
}

export interface ParamSpec {
  name: string;
  type: 'int' | 'float' | 'string';
  min?: number;
  max?: number;
  default?: number | string | boolean;
  required?: boolean;
  description?: string;
}

export interface KnobDefinition {
  key: string;
  service: string;
  kind: string;
  description: string;
  params: ParamSpec[];
  guidance?: string;
}

export interface ActiveResponse {
  active: Knob | null;
  updatedAt: string;
}

export interface Event {
  type: 'activate' | 'clear';
  knob?: Knob;
  previous?: Knob;
  at: string;
}

const BASE = '/admin/faults';

export async function getActive(): Promise<ActiveResponse> {
  const r = await fetch(BASE);
  if (!r.ok) throw new Error(`getActive: ${r.status}`);
  return r.json();
}

export async function getCatalog(): Promise<KnobDefinition[]> {
  const r = await fetch(`${BASE}/catalog`);
  if (!r.ok) throw new Error(`getCatalog: ${r.status}`);
  return r.json();
}

export async function activate(knob: Partial<Knob> & { key: string }): Promise<Knob> {
  const r = await fetch(BASE, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(knob),
  });
  if (!r.ok) throw new Error(`activate ${knob.key}: ${r.status} ${await r.text()}`);
  return r.json();
}

export async function clearActive(): Promise<void> {
  const r = await fetch(BASE, { method: 'DELETE' });
  if (!r.ok) throw new Error(`clear: ${r.status}`);
}

// subscribeEvents opens an SSE stream that replays the recent event ring
// then forwards live events. Returns an unsubscribe function.
export function subscribeEvents(onEvent: (e: Event) => void): () => void {
  const es = new EventSource(`${BASE}/events`);
  const handle = (ev: MessageEvent) => {
    try {
      onEvent(JSON.parse(ev.data));
    } catch {
      /* ignore malformed */
    }
  };
  es.addEventListener('activate', handle);
  es.addEventListener('clear', handle);
  return () => es.close();
}
