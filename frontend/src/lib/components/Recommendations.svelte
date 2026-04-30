<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { api, type Product } from '../api';
  import { cart } from '../stores/cart';
  import ProductImage from './ProductImage.svelte';

  export let productId: string | null = null;
  export let title = 'Recommended for You';

  let recommendations: Product[] = [];
  let loading = true;

  const dispatch = createEventDispatcher();

  onMount(async () => {
    try {
      if (productId) {
        recommendations = await api.getProductRecommendations(productId, 4);
        title = 'You Might Also Like';
      } else {
        recommendations = await api.getRecommendations(4);
      }
    } catch (error) {
      console.error('Failed to load recommendations:', error);
      // Fallback to random products
      try {
        const allProducts = await api.getProducts();
        recommendations = allProducts
          .sort(() => Math.random() - 0.5)
          .slice(0, 4);
      } catch (e) {
        console.error('Failed to load fallback products:', e);
      }
    } finally {
      loading = false;
    }
  });

  async function handleAddToCart(product: Product) {
    try {
      await cart.addItem(product.id);
      // Show feedback
      const button = document.querySelector(`[data-rec-product="${product.id}"]`);
      if (button) {
        button.textContent = '✓ Added';
        button.classList.add('bg-green-600');
        setTimeout(() => {
          button.textContent = 'Quick Add';
          button.classList.remove('bg-green-600');
        }, 1000);
      }
    } catch (err) {
      console.error('Failed to add to cart:', err);
    }
  }

  function handleProductClick(product: Product) {
    dispatch('select', product);
  }

</script>

<div class="py-8">
  <h3 class="text-2xl font-bold text-gray-900 mb-6">{title}</h3>

  {#if loading}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [1, 2, 3, 4] as _}
        <div class="bg-white rounded-lg shadow-sm p-4 animate-pulse">
          <div class="aspect-square bg-gray-200 rounded-lg mb-3"></div>
          <div class="h-4 bg-gray-200 rounded mb-2"></div>
          <div class="h-3 bg-gray-200 rounded w-3/4"></div>
        </div>
      {/each}
    </div>
  {:else if recommendations.length > 0}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each recommendations as product (product.id)}
        <div class="bg-white rounded-lg shadow-sm hover:shadow-md transition-shadow cursor-pointer group" on:click={() => handleProductClick(product)}>
          <div class="aspect-square rounded-t-lg overflow-hidden group-hover:scale-105 transition-transform">
            <ProductImage
              productId={product.id}
              productName={product.name}
              size="large"
              className="w-full h-full"
            />
          </div>
          <div class="p-3">
            <h4 class="font-medium text-sm text-gray-900 line-clamp-1">{product.name}</h4>
            <p class="text-xs text-gray-600 mt-1 line-clamp-2">{product.description}</p>
            <div class="mt-3 flex items-center justify-between">
              <span class="text-lg font-bold text-gray-900">${product.price.toFixed(2)}</span>
              <button
                data-rec-product={product.id}
                on:click|stopPropagation={() => handleAddToCart(product)}
                disabled={product.stock === 0}
                class="bg-purple-600 text-white px-3 py-1 text-sm rounded hover:bg-purple-700 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
              >
                {product.stock === 0 ? 'Out' : 'Quick Add'}
              </button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <p class="text-gray-500 text-center">No recommendations available</p>
  {/if}
</div>