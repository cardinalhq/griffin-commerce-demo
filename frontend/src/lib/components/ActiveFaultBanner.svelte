{#if active}
  <div class="bg-amber-100 border-y border-amber-300 px-4 py-2 text-amber-900">
    <div class="max-w-6xl mx-auto flex items-center gap-3 text-sm">
      <span class="font-bold">⚠ Fault active:</span>
      <code class="bg-amber-200 px-2 py-0.5 rounded font-mono">{active.key}</code>
      <span class="text-amber-800">
        {#if active.target}target=<b>{active.target}</b>{/if}
        {#if active.probability}probability=<b>{active.probability}</b>{/if}
        {#if active.latencyMs}latency=<b>{active.latencyMs}ms</b>{/if}
        {#if active.statusCode}status=<b>{active.statusCode}</b>{/if}
      </span>
      <span class="ml-auto text-xs text-amber-700">started {timeAgo(active.startedAt)}</span>
    </div>
  </div>
{/if}

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getActive, subscribeEvents, type Knob } from '../faults';

  let active: Knob | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let unsubscribe: (() => void) | null = null;

  async function refresh() {
    try {
      const r = await getActive();
      active = r.active;
    } catch {
      // Control plane unreachable; keep last value, banner just stays as-is.
    }
  }

  function timeAgo(rfc: string): string {
    const ms = Date.now() - new Date(rfc).getTime();
    if (ms < 60_000) return `${Math.round(ms / 1000)}s ago`;
    return `${Math.round(ms / 60_000)}m ago`;
  }

  onMount(() => {
    refresh();
    // Poll as a backstop in case SSE drops; SSE handles live updates.
    pollTimer = setInterval(refresh, 5000);
    unsubscribe = subscribeEvents((ev) => {
      if (ev.type === 'activate') active = ev.knob ?? null;
      else if (ev.type === 'clear') active = null;
    });
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
    if (unsubscribe) unsubscribe();
  });
</script>
