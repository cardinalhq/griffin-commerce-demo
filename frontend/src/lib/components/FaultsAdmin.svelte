<div class="max-w-6xl mx-auto p-6">
  <h1 class="text-3xl font-bold mb-2">Fault Injection Admin</h1>
  <p class="text-gray-600 mb-6">Activate one knob at a time to inject a fault into the demo. Switching to a new knob automatically replaces the previous one.</p>

  <!-- Active fault card -->
  <div class="border rounded-lg p-4 mb-6" class:bg-amber-50={active} class:border-amber-300={active} class:bg-gray-50={!active}>
    <div class="flex justify-between items-start">
      <div>
        <h2 class="text-lg font-semibold mb-1">
          {active ? `Active: ${active.key}` : 'No fault active'}
        </h2>
        {#if active}
          <p class="text-sm text-gray-700">
            <span class="mr-3">service=<code>{active.service}</code></span>
            <span class="mr-3">kind=<code>{active.kind}</code></span>
            {#if active.target}<span class="mr-3">target=<code>{active.target}</code></span>{/if}
            {#if active.probability}<span class="mr-3">probability=<code>{active.probability}</code></span>{/if}
            {#if active.latencyMs}<span class="mr-3">latencyMs=<code>{active.latencyMs}</code></span>{/if}
            {#if active.statusCode}<span class="mr-3">statusCode=<code>{active.statusCode}</code></span>{/if}
          </p>
          <p class="text-xs text-gray-500 mt-1">started at {active.startedAt}</p>
        {/if}
      </div>
      {#if active}
        <button on:click={onClear} class="bg-red-500 hover:bg-red-600 text-white px-4 py-2 rounded font-medium">Clear</button>
      {/if}
    </div>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
    <!-- Knob catalog -->
    <div class="lg:col-span-2 space-y-3">
      <h2 class="text-xl font-semibold">Available knobs</h2>
      {#if catalog.length === 0}
        <p class="text-gray-500 italic">Loading…</p>
      {:else}
        {#each catalog as def (def.key)}
          <div class="border rounded p-4 bg-white" class:ring-2={active?.key === def.key} class:ring-amber-400={active?.key === def.key}>
            <div class="flex justify-between items-start mb-2">
              <div>
                <h3 class="font-mono text-sm font-bold text-gray-900">{def.key}</h3>
                <span class="inline-block text-xs bg-gray-100 text-gray-700 rounded px-2 py-0.5 mr-1">{def.service}</span>
                <span class="inline-block text-xs bg-gray-100 text-gray-700 rounded px-2 py-0.5">{def.kind}</span>
              </div>
              <button on:click={() => onActivate(def)} disabled={busy} class="bg-blue-500 hover:bg-blue-600 disabled:bg-gray-300 text-white px-3 py-1 rounded text-sm font-medium">
                Activate
              </button>
            </div>
            <p class="text-sm text-gray-700 mb-2">{def.description}</p>
            {#if def.guidance}
              <p class="text-xs text-gray-500 italic mb-2">{def.guidance}</p>
            {/if}
            {#if def.params.length > 0}
              <div class="grid grid-cols-2 gap-2 mt-2">
                {#each def.params as param}
                  <label class="text-xs">
                    <span class="block text-gray-600 mb-0.5">
                      {param.name}{param.required ? ' *' : ''}
                      {#if param.min !== undefined && param.max !== undefined}
                        <span class="text-gray-400">[{param.min}–{param.max}]</span>
                      {/if}
                    </span>
                    <input
                      type={param.type === 'string' ? 'text' : 'number'}
                      bind:value={paramValues[def.key][param.name]}
                      step={param.type === 'float' ? '0.01' : '1'}
                      placeholder={param.default !== undefined ? String(param.default) : ''}
                      class="w-full border rounded px-2 py-1 font-mono text-sm"
                    />
                    {#if param.description}
                      <span class="block text-gray-400 text-xs mt-0.5">{param.description}</span>
                    {/if}
                  </label>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    <!-- Event log -->
    <div>
      <h2 class="text-xl font-semibold mb-2">Event log</h2>
      <div class="border rounded p-3 bg-gray-50 max-h-[600px] overflow-y-auto text-xs font-mono space-y-1">
        {#if events.length === 0}
          <p class="text-gray-400">no events yet</p>
        {:else}
          {#each events.slice().reverse() as ev}
            <div>
              <span class="text-gray-500">{new Date(ev.at).toLocaleTimeString()}</span>
              {#if ev.type === 'activate'}
                <span class="text-green-700 font-bold">▶ activate</span>
                <code>{ev.knob?.key}</code>
              {:else}
                <span class="text-red-700 font-bold">■ clear</span>
                <code>{ev.previous?.key ?? ''}</code>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
      {#if errorMsg}
        <p class="text-red-600 text-sm mt-2">{errorMsg}</p>
      {/if}
    </div>
  </div>
</div>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    getActive,
    getCatalog,
    activate,
    clearActive,
    subscribeEvents,
    type Knob,
    type KnobDefinition,
    type Event,
  } from '../faults';

  let active: Knob | null = null;
  let catalog: KnobDefinition[] = [];
  let events: Event[] = [];
  let busy = false;
  let errorMsg = '';
  let unsubscribe: (() => void) | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  // paramValues[knobKey][paramName] holds the current input value for each
  // parameter input. Initialized from defaults when the catalog loads.
  let paramValues: Record<string, Record<string, any>> = {};

  async function loadInitial() {
    try {
      catalog = await getCatalog();
      paramValues = {};
      for (const def of catalog) {
        paramValues[def.key] = {};
        for (const p of def.params) {
          paramValues[def.key][p.name] = p.default ?? '';
        }
      }
      const a = await getActive();
      active = a.active;
    } catch (err) {
      errorMsg = String(err);
    }
  }

  async function onActivate(def: KnobDefinition) {
    busy = true;
    errorMsg = '';
    try {
      const knob: Partial<Knob> & { key: string } = { key: def.key };
      const params = paramValues[def.key] ?? {};
      for (const p of def.params) {
        const raw = params[p.name];
        if (raw === '' || raw === null || raw === undefined) continue;
        switch (p.name) {
          case 'target':       (knob as any).target = String(raw); break;
          case 'probability':  (knob as any).probability = Number(raw); break;
          case 'latencyMs':    (knob as any).latencyMs = Number(raw); break;
          case 'statusCode':   (knob as any).statusCode = Number(raw); break;
        }
      }
      const stored = await activate(knob);
      active = stored;
    } catch (err) {
      errorMsg = String(err);
    } finally {
      busy = false;
    }
  }

  async function onClear() {
    busy = true;
    errorMsg = '';
    try {
      await clearActive();
      active = null;
    } catch (err) {
      errorMsg = String(err);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    loadInitial();
    unsubscribe = subscribeEvents((ev) => {
      events = [...events, ev];
      if (ev.type === 'activate') active = ev.knob ?? null;
      else if (ev.type === 'clear') active = null;
    });
    // Polling backstop in case SSE drops.
    pollTimer = setInterval(async () => {
      try {
        const a = await getActive();
        active = a.active;
      } catch {}
    }, 5000);
  });

  onDestroy(() => {
    if (unsubscribe) unsubscribe();
    if (pollTimer) clearInterval(pollTimer);
  });
</script>
