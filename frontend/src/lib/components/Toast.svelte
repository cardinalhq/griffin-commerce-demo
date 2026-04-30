<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  import { toasts } from '../stores/toast';

  function bgClass(kind: string): string {
    switch (kind) {
      case 'error':   return 'bg-red-600';
      case 'success': return 'bg-green-600';
      default:        return 'bg-gray-800';
    }
  }
</script>

<div
  class="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none"
  role="status"
  aria-live="polite"
>
  {#each $toasts as toast (toast.id)}
    <div
      class="pointer-events-auto min-w-[260px] max-w-sm shadow-lg rounded-lg text-white px-4 py-3 flex items-start gap-3 {bgClass(toast.kind)}"
      role="alert"
    >
      <span class="flex-1 text-sm leading-snug">{toast.text}</span>
      <button
        class="opacity-80 hover:opacity-100 text-white/90 leading-none text-lg"
        aria-label="Dismiss notification"
        on:click={() => toasts.dismiss(toast.id)}
      >×</button>
    </div>
  {/each}
</div>
