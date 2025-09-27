<script lang="ts">
  import { fade } from 'svelte/transition';
  import { cart } from '../stores/cart';
  import ProductImage from './ProductImage.svelte';

  export let product: any = null;
  export let show = false;

  async function handleAddToCart() {
    if (!product) return;

    try {
      await cart.addItem(product.id);

      // Show success feedback
      const button = document.querySelector(`[data-modal-product-id="${product.id}"]`);
      if (button) {
        button.textContent = '✓ Added to Cart';
        button.classList.add('bg-green-600');
        setTimeout(() => {
          button.textContent = 'Add to Cart';
          button.classList.remove('bg-green-600');
        }, 1500);
      }
    } catch (err) {
      console.error('Failed to add to cart:', err);
      alert('Failed to add item to cart. Please try again.');
    }
  }

  function getEmoji(productName: string) {
    const name = productName.toLowerCase();
    if (name.includes('food')) return '🍖';
    if (name.includes('toy')) return '🎾';
    if (name.includes('bed')) return '🛏️';
    if (name.includes('collar')) return '🔵';
    if (name.includes('ball')) return '⚾';
    if (name.includes('shampoo')) return '🧴';
    return '📦';
  }
</script>

{#if show && product}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black bg-opacity-50 z-40"
    on:click={() => show = false}
    transition:fade
  ></div>

  <!-- Product Detail Modal -->
  <div
    class="fixed inset-0 flex items-center justify-center z-50 p-4"
    on:click={() => show = false}
    transition:fade
  >
    <div
      class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto"
      on:click|stopPropagation>
      <div class="relative">
        <!-- Close button -->
        <button
          on:click={() => show = false}
          class="absolute top-4 right-4 text-gray-500 hover:text-gray-700 z-10"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        <!-- Product Image -->
        <div class="h-64 rounded-t-lg overflow-hidden">
          <ProductImage
            productId={product.id}
            productName={product.name}
            size="large"
            className="w-full h-full"
          />
        </div>

        <!-- Product Info -->
        <div class="p-6">
          <div class="mb-4">
            <h2 class="text-2xl font-bold text-gray-900 mb-2">{product.name}</h2>
            <p class="text-gray-600">{product.description}</p>
          </div>

          <div class="grid grid-cols-2 gap-4 mb-6">
            <div class="bg-gray-50 p-3 rounded-lg">
              <p class="text-sm text-gray-500 mb-1">Price</p>
              <p class="text-xl font-bold text-gray-900">${product.price.toFixed(2)}</p>
            </div>
            <div class="bg-gray-50 p-3 rounded-lg">
              <p class="text-sm text-gray-500 mb-1">Stock</p>
              <p class="text-xl font-bold text-gray-900">
                {#if product.stock > 0}
                  {product.stock} available
                  {#if product.stock < 10}
                    <span class="text-sm text-orange-600 block">Low stock!</span>
                  {/if}
                {:else}
                  <span class="text-red-600">Out of stock</span>
                {/if}
              </p>
            </div>
          </div>

          <!-- Product Details -->
          <div class="border-t pt-4 mb-6">
            <h3 class="font-semibold mb-3">Product Details</h3>
            <ul class="space-y-2 text-sm text-gray-600">
              <li class="flex items-start">
                <svg class="w-5 h-5 text-purple-600 mr-2 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                <span>High-quality product from trusted brands</span>
              </li>
              <li class="flex items-start">
                <svg class="w-5 h-5 text-purple-600 mr-2 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                <span>Safe for all dogs and puppies</span>
              </li>
              <li class="flex items-start">
                <svg class="w-5 h-5 text-purple-600 mr-2 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                <span>30-day satisfaction guarantee</span>
              </li>
              <li class="flex items-start">
                <svg class="w-5 h-5 text-purple-600 mr-2 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                <span>Fast shipping available</span>
              </li>
            </ul>
          </div>

          <!-- Add to Cart Button -->
          <button
            data-modal-product-id={product.id}
            on:click={handleAddToCart}
            disabled={product.stock === 0}
            class="w-full bg-purple-600 text-white py-3 px-4 rounded-lg hover:bg-purple-700 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed flex items-center justify-center"
          >
            {#if product.stock === 0}
              Out of Stock
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z" />
              </svg>
              Add to Cart
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}