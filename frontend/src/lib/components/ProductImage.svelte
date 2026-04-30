<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  export let productId: string;
  export let productName: string;
  export let size: 'small' | 'medium' | 'large' = 'medium';
  export let className: string = '';

  let imageUrl: string | null = null;
  let imageOffset: string = 'center';
  let loading = true;
  let error = false;

  const sizeClasses = {
    small: 'h-16 w-16 text-2xl',
    medium: 'h-32 w-32 text-5xl',
    large: 'h-64 w-64 text-8xl'
  };

  function getEmoji(name: string): string {
    const lowerName = name.toLowerCase();
    if (lowerName.includes('food')) return '🍖';
    if (lowerName.includes('toy') || lowerName.includes('rope')) return '⚾';
    if (lowerName.includes('bed')) return '🛏️';
    if (lowerName.includes('collar')) return '🔵';
    if (lowerName.includes('ball') || lowerName.includes('tennis')) return '🎾';
    if (lowerName.includes('shampoo')) return '🧴';
    if (lowerName.includes('treat')) return '🍪';
    if (lowerName.includes('leash')) return '🦴';
    return '📦';
  }

  // Re-run whenever productId changes. Capture id at fetch time so a slow
  // response from a previous productId can't overwrite the current image.
  $: if (productId) loadImage(productId);

  async function loadImage(id: string) {
    loading = true;
    error = false;
    imageUrl = null;
    try {
      const response = await fetch(`/api/images/product/${id}`);
      if (id !== productId) return;
      if (!response.ok) {
        error = true;
        loading = false;
        return;
      }
      const data = await response.json();
      if (id !== productId) return;

      imageOffset = data.offset || 'center';
      const cacheBuster = data.hash ? `v=${data.hash}` : `t=${Date.now()}`;
      const url = `${data.image_url}?${cacheBuster}`;

      const img = new Image();
      img.onload = () => {
        if (id !== productId) return;
        imageUrl = url;
        loading = false;
      };
      img.onerror = () => {
        if (id !== productId) return;
        error = true;
        loading = false;
      };
      img.src = url;
    } catch (err) {
      if (id !== productId) return;
      console.error('Failed to fetch product image:', err);
      error = true;
      loading = false;
    }
  }
</script>

<div class="{sizeClasses[size]} {className} flex items-center justify-center bg-gradient-to-br from-purple-100 to-pink-100 rounded-lg overflow-hidden">
  {#if loading}
    <div class="animate-pulse bg-gray-200 w-full h-full"></div>
  {:else if error || !imageUrl}
    <span class="select-none">{getEmoji(productName)}</span>
  {:else}
    <img
      src={imageUrl}
      alt={productName}
      class="w-full h-full object-cover"
      style="object-position: {imageOffset}"
      on:error={() => { error = true; imageUrl = null; }}
    />
  {/if}
</div>
