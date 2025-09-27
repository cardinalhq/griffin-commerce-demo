<script lang="ts">
  import { cart, cartTotal } from '../stores/cart';
  import { fly } from 'svelte/transition';
  import EnhancedCheckout from './EnhancedCheckout.svelte';

  export let show = false;
  let showCheckout = false;
</script>

{#if show}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black bg-opacity-50 z-40"
    on:click={() => show = false}
  ></div>

  <!-- Cart Drawer -->
  <div
    class="fixed right-0 top-0 h-full w-96 bg-white shadow-lg z-50"
    transition:fly={{ x: 400, duration: 300 }}
  >
    <div class="p-4 border-b">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-semibold">Shopping Cart</h2>
        <button
          on:click={() => show = false}
          class="text-gray-500 hover:text-gray-700"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#if $cart && $cart.items.length > 0}
        <div class="space-y-4">
          {#each $cart.items as item}
            <div class="flex items-center justify-between border-b pb-4">
              <div class="flex-1">
                <h3 class="font-medium">{item.name}</h3>
                <p class="text-sm text-gray-600">
                  ${item.price.toFixed(2)} each
                </p>
              </div>
              <div class="flex items-center space-x-3">
                <div class="flex items-center border rounded-lg">
                  <button
                    on:click={() => cart.updateQuantity(item.product_id, item.quantity - 1)}
                    class="px-3 py-1 hover:bg-gray-100 transition-colors"
                    disabled={item.quantity <= 1}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" />
                    </svg>
                  </button>
                  <span class="px-3 py-1 min-w-[3rem] text-center">{item.quantity}</span>
                  <button
                    on:click={() => cart.updateQuantity(item.product_id, item.quantity + 1)}
                    class="px-3 py-1 hover:bg-gray-100 transition-colors"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                    </svg>
                  </button>
                </div>
                <span class="font-semibold min-w-[4rem] text-right">${item.subtotal.toFixed(2)}</span>
                <button
                  on:click={() => cart.removeItem(item.product_id)}
                  class="text-red-500 hover:text-red-700"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </div>
          {/each}
        </div>

        <div class="mt-6 border-t pt-4">
          <div class="flex justify-between text-lg font-semibold">
            <span>Total:</span>
            <span>${$cartTotal.toFixed(2)}</span>
          </div>
          <button
            class="w-full mt-4 bg-purple-600 text-white py-3 px-4 rounded-lg hover:bg-purple-700 transition-colors"
            on:click={() => { showCheckout = true; show = false; }}
          >
            Proceed to Checkout
          </button>
          <button
            class="w-full mt-2 text-gray-600 hover:text-gray-800"
            on:click={() => cart.clear()}
          >
            Clear Cart
          </button>
        </div>
      {:else}
        <div class="text-center py-8">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-24 w-24 text-gray-300 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
          <p class="text-gray-500">Your cart is empty</p>
        </div>
      {/if}
    </div>
  </div>
{/if}

<EnhancedCheckout bind:show={showCheckout} />