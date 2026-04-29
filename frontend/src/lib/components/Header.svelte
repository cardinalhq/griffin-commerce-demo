<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  import { cartCount } from '../stores/cart';
  import Cart from './Cart.svelte';

  let showCart = false;
</script>

<header class="bg-white shadow-sm border-b sticky top-0 z-40">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="flex items-center justify-between h-16">
      <div class="flex items-center">
        <h1 class="text-2xl font-bold text-gray-900">
          <a href="/" class="flex items-center space-x-2">
            <span>🐕</span>
            <span>Griffin Commerce</span>
          </a>
        </h1>
      </div>

      <nav class="flex items-center space-x-6">
        <a href="/" class="text-gray-600 hover:text-gray-900">Products</a>

        <button
          on:click={() => showCart = !showCart}
          class="relative text-gray-600 hover:text-gray-900"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
          {#if $cartCount > 0}
            <span class="absolute -top-2 -right-2 bg-purple-600 text-white text-xs rounded-full w-5 h-5 flex items-center justify-center">
              {$cartCount}
            </span>
          {/if}
        </button>
      </nav>
    </div>
  </div>
</header>

<Cart bind:show={showCart} />