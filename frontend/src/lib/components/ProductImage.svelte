<script lang="ts">
  import { onMount } from 'svelte';

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
    if (lowerName.includes('toy') || lowerName.includes('rope')) return '🎾';
    if (lowerName.includes('bed')) return '🛏️';
    if (lowerName.includes('collar')) return '🔵';
    if (lowerName.includes('ball') || lowerName.includes('tennis')) return '⚾';
    if (lowerName.includes('shampoo')) return '🧴';
    if (lowerName.includes('treat')) return '🍪';
    if (lowerName.includes('leash')) return '🦴';
    return '📦';
  }

  onMount(async () => {
    try {
      const response = await fetch(`http://localhost:8083/api/images/product/${productId}`);
      if (response.ok) {
        const data = await response.json();

        // Store offset from API response
        imageOffset = data.offset || 'center';

        // Try to load the actual image
        const img = new Image();
        img.onload = () => {
          // Use hash for cache busting if available, otherwise use timestamp
          const cacheBuster = data.hash ? `v=${data.hash}` : `t=${Date.now()}`;
          imageUrl = `http://localhost:8083${data.image_url}?${cacheBuster}`;
          loading = false;
        };
        img.onerror = () => {
          // Image failed to load, use emoji fallback
          error = true;
          loading = false;
        };
        // Use hash for cache busting when checking if image loads
        const cacheBuster = data.hash ? `v=${data.hash}` : `t=${Date.now()}`;
        img.src = `http://localhost:8083${data.image_url}?${cacheBuster}`;
      } else {
        error = true;
        loading = false;
      }
    } catch (err) {
      console.error('Failed to fetch product image:', err);
      error = true;
      loading = false;
    }
  });
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