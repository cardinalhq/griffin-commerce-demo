<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './lib/components/Header.svelte';
  import ProductDetail from './lib/components/ProductDetail.svelte';
  import { cart } from './lib/stores/cart';

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

  // Initialize cart
  onMount(async () => {
    await cart.init();

    try {
      const response = await fetch('http://localhost:8080/api/products');
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
      alert('Failed to add item to cart. Please try again.');
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
        {#each filteredProducts as product}
          <div class="bg-white rounded-lg shadow-sm hover:shadow-md transition-shadow cursor-pointer group" on:click={() => openProductDetail(product)}>
            <div class="aspect-square bg-gradient-to-br from-purple-100 to-pink-100 rounded-t-lg flex items-center justify-center text-6xl group-hover:scale-110 transition-transform">
              {#if product.name.toLowerCase().includes('food')}
                🍖
              {:else if product.name.toLowerCase().includes('toy')}
                🎾
              {:else if product.name.toLowerCase().includes('bed')}
                🛏️
              {:else if product.name.toLowerCase().includes('collar')}
                🔵
              {:else if product.name.toLowerCase().includes('ball')}
                ⚾
              {:else if product.name.toLowerCase().includes('shampoo')}
                🧴
              {:else}
                📦
              {/if}
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
    {/if}
  </main>
</div>

<ProductDetail bind:show={showProductDetail} product={selectedProduct} />