// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

import { writable } from 'svelte/store';

export type ToastKind = 'error' | 'success' | 'info';

export interface ToastMessage {
  id: number;
  kind: ToastKind;
  text: string;
}

function createToastStore() {
  const { subscribe, update } = writable<ToastMessage[]>([]);
  let nextId = 1;

  function show(text: string, kind: ToastKind = 'info', durationMs = 4000) {
    const id = nextId++;
    update((toasts) => [...toasts, { id, kind, text }]);
    setTimeout(() => {
      update((toasts) => toasts.filter((t) => t.id !== id));
    }, durationMs);
  }

  function dismiss(id: number) {
    update((toasts) => toasts.filter((t) => t.id !== id));
  }

  return {
    subscribe,
    show,
    error: (text: string) => show(text, 'error'),
    success: (text: string) => show(text, 'success'),
    info: (text: string) => show(text, 'info'),
    dismiss,
  };
}

export const toasts = createToastStore();
