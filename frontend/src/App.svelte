<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/Header.svelte';
  import ProductDetail from './lib/components/ProductDetail.svelte';
  import ProductImage from './lib/components/ProductImage.svelte';
  import Recommendations from './lib/components/Recommendations.svelte';
  import Toast from './lib/components/Toast.svelte';
  import { cart } from './lib/stores/cart';
  import { toasts } from './lib/stores/toast';

  interface Product {
    id: string;
    name: string;
    description: string;
    price: number;
    stock: number;
    category?: string;
  }

  let products: Product[] = [];
  let loading = true;
  let error: string | null = null;
  let selectedProduct: Product | null = null;
  let showProductDetail = false;
  let searchQuery = '';
  let selectedCategory = 'all';
  let currentPage = 1;
  let productsPerPage = 6;

  // Initialize cart
  onMount(async () => {
    await cart.init();

    try {
      const response = await fetch('/api/products');
      if (!response.ok) throw new Error('Failed to fetch products');
      const data = await response.json();
      products = Array.isArray(data) ? data : (data.products || []);
    } catch (err) {
      error = err instanceof Error ? err.message : 'An error occurred';
    } finally {
      loading = false;
    }
  });

  async function handleAddToCart(product: Product) {
    try {
      await cart.addItem(product.id);
      // Show success feedback
      const button = document.querySelector(`[data-product-id="${product.id}"]`);
      if (button) {
        button.textContent = '✓ Added';
        button.classList.add('bg-green-600');
        setTimeout(() => {
          button.textContent = 'Add to Cart';
          button.classList.remove('bg-green-600');
        }, 1000);
      }
    } catch (err) {
      console.error('Failed to add to cart:', err);
      toasts.error('Failed to add item to cart. Please try again.');
    }
  }

  function openProductDetail(product: Product) {
    selectedProduct = product;
    showProductDetail = true;
  }

  // Get unique categories
  $: categories = ['all', ...new Set(products.map(p => p.category).filter(Boolean))];

  // Filter products based on search and category
  $: filteredProducts = products.filter(product => {
    const matchesSearch = product.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                          (product.description && product.description.toLowerCase().includes(searchQuery.toLowerCase()));
    const matchesCategory = selectedCategory === 'all' || product.category === selectedCategory;
    return matchesSearch && matchesCategory;
  });

  // Calculate pagination
  $: totalPages = Math.ceil(filteredProducts.length / productsPerPage);
  $: paginatedProducts = filteredProducts.slice(
    (currentPage - 1) * productsPerPage,
    currentPage * productsPerPage
  );

  // Reset to page 1 when filters change
  $: searchQuery, selectedCategory, currentPage = 1;
</script>

<div class="min-h-screen bg-gray-50">
  <Header />

  <!-- Main Content -->
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <!-- Search and Filter Bar -->
    <div class="mb-8">
      <div class="flex flex-col sm:flex-row gap-4">
        <div class="flex-1">
          <input
            type="text"
            bind:value={searchQuery}
            placeholder="Search products..."
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        </div>
        <select
          bind:value={selectedCategory}
          class="px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
        >
          {#each categories as category}
            <option value={category}>
              {category === 'all' ? 'All Categories' : category}
            </option>
          {/each}
        </select>
      </div>
    </div>

    <h2 class="text-3xl font-bold text-gray-900 mb-8">Featured Products</h2>

    {#if loading}
      <div class="flex justify-center items-center h-64">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-600"></div>
      </div>
    {:else if error}
      <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
        Error: {error}
      </div>
    {:else if filteredProducts.length === 0}
      <p class="text-gray-500">No products found</p>
    {:else}
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {#each paginatedProducts as product}
          <div class="bg-white rounded-lg shadow-sm hover:shadow-md transition-shadow cursor-pointer group" on:click={() => openProductDetail(product)}>
            <div class="aspect-square rounded-t-lg overflow-hidden group-hover:scale-105 transition-transform">
              <ProductImage
                productId={product.id}
                productName={product.name}
                size="large"
                className="w-full h-full"
              />
            </div>
            <div class="p-4">
              <h3 class="font-semibold text-gray-900">{product.name}</h3>
              <p class="text-sm text-gray-600 mt-1">{product.description}</p>
              <div class="mt-4 flex items-center justify-between">
                <span class="text-xl font-bold text-gray-900">${product.price.toFixed(2)}</span>
                <button
                  data-product-id={product.id}
                  on:click|stopPropagation={() => handleAddToCart(product)}
                  disabled={product.stock === 0}
                  class="bg-purple-600 text-white px-4 py-2 rounded-lg hover:bg-purple-700 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
                >
                  {product.stock === 0 ? 'Out of Stock' : 'Add to Cart'}
                </button>
              </div>
              {#if product.stock > 0 && product.stock < 10}
                <p class="text-sm text-orange-600 mt-2">Only {product.stock} left!</p>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <!-- Pagination Controls -->
      {#if totalPages > 1}
        <div class="mt-8 flex justify-center items-center space-x-2">
          <button
            on:click={() => currentPage = Math.max(1, currentPage - 1)}
            disabled={currentPage === 1}
            class="px-3 py-2 rounded-lg border border-gray-300 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clip-rule="evenodd" />
            </svg>
          </button>

          {#if totalPages <= 7}
            {#each Array(totalPages) as _, i}
              <button
                on:click={() => currentPage = i + 1}
                class="px-3 py-2 rounded-lg {currentPage === i + 1 ? 'bg-purple-600 text-white' : 'border border-gray-300 hover:bg-gray-100'}"
              >
                {i + 1}
              </button>
            {/each}
          {:else}
            {#if currentPage > 2}
              <button on:click={() => currentPage = 1} class="px-3 py-2 rounded-lg border border-gray-300 hover:bg-gray-100">1</button>
              {#if currentPage > 3}
                <span class="px-2">...</span>
              {/if}
            {/if}

            {#each Array(3) as _, i}
              {#if currentPage - 1 + i > 0 && currentPage - 1 + i <= totalPages}
                <button
                  on:click={() => currentPage = currentPage - 1 + i}
                  class="px-3 py-2 rounded-lg {i === 1 ? 'bg-purple-600 text-white' : 'border border-gray-300 hover:bg-gray-100'}"
                >
                  {currentPage - 1 + i}
                </button>
              {/if}
            {/each}

            {#if currentPage < totalPages - 1}
              {#if currentPage < totalPages - 2}
                <span class="px-2">...</span>
              {/if}
              <button on:click={() => currentPage = totalPages} class="px-3 py-2 rounded-lg border border-gray-300 hover:bg-gray-100">{totalPages}</button>
            {/if}
          {/if}

          <button
            on:click={() => currentPage = Math.min(totalPages, currentPage + 1)}
            disabled={currentPage === totalPages}
            class="px-3 py-2 rounded-lg border border-gray-300 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
            </svg>
          </button>
        </div>

        <div class="text-center mt-2 text-sm text-gray-600">
          Showing {(currentPage - 1) * productsPerPage + 1} - {Math.min(currentPage * productsPerPage, filteredProducts.length)} of {filteredProducts.length} products
        </div>
      {/if}
    {/if}

    <!-- Recommendations Section -->
    {#if !searchQuery && selectedCategory === 'all'}
      <Recommendations on:select={(e) => openProductDetail(e.detail)} />
    {/if}
  </main>
</div>

<ProductDetail bind:show={showProductDetail} product={selectedProduct} />

<Toast />