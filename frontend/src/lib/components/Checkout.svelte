<script lang="ts">
  import { cart, cartTotal } from '../stores/cart';
  import { fade } from 'svelte/transition';

  export let show = false;

  let customerInfo = {
    name: '',
    email: '',
    address: '',
    city: '',
    zipCode: '',
    cardNumber: '',
    expiryDate: '',
    cvv: ''
  };

  let processing = false;
  let orderComplete = false;
  let orderId = '';

  async function handleCheckout() {
    processing = true;

    try {
      // Simulate checkout process
      const response = await fetch(`http://localhost:8082/api/cart/${$cart?.id}/checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(customerInfo)
      });

      if (response.ok) {
        const result = await response.json();
        orderId = `ORD-${Date.now()}`;
        orderComplete = true;

        // Clear cart after successful checkout
        setTimeout(() => {
          cart.reset();
          show = false;
          orderComplete = false;
          processing = false;
          // Reset form
          customerInfo = {
            name: '',
            email: '',
            address: '',
            city: '',
            zipCode: '',
            cardNumber: '',
            expiryDate: '',
            cvv: ''
          };
        }, 3000);
      }
    } catch (error) {
      console.error('Checkout failed:', error);
      alert('Checkout failed. Please try again.');
      processing = false;
    }
  }

  function formatCardNumber(value: string) {
    const v = value.replace(/\s+/g, '').replace(/[^0-9]/gi, '');
    const matches = v.match(/\d{4,16}/g);
    const match = (matches && matches[0]) || '';
    const parts = [];

    for (let i = 0, len = match.length; i < len; i += 4) {
      parts.push(match.substring(i, i + 4));
    }

    if (parts.length) {
      return parts.join(' ');
    } else {
      return value;
    }
  }
</script>

{#if show}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black bg-opacity-50 z-40"
    on:click={() => !processing && (show = false)}
    transition:fade
  ></div>

  <!-- Checkout Modal -->
  <div
    class="fixed inset-0 flex items-center justify-center z-50 p-4"
    transition:fade
  >
    <div class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
      {#if orderComplete}
        <div class="p-8 text-center">
          <div class="mb-4">
            <div class="inline-flex items-center justify-center w-16 h-16 bg-green-100 rounded-full mb-4">
              <svg class="w-8 h-8 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
              </svg>
            </div>
          </div>
          <h2 class="text-2xl font-bold text-gray-900 mb-2">Order Confirmed!</h2>
          <p class="text-gray-600 mb-4">Thank you for your purchase</p>
          <p class="text-sm text-gray-500">Order ID: {orderId}</p>
        </div>
      {:else}
        <div class="p-6 border-b">
          <div class="flex items-center justify-between">
            <h2 class="text-2xl font-bold">Checkout</h2>
            <button
              on:click={() => show = false}
              disabled={processing}
              class="text-gray-500 hover:text-gray-700 disabled:opacity-50"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <form on:submit|preventDefault={handleCheckout} class="p-6 space-y-6">
          <!-- Order Summary -->
          <div class="bg-gray-50 p-4 rounded-lg">
            <h3 class="font-semibold mb-3">Order Summary</h3>
            {#if $cart && $cart.items.length > 0}
              <div class="space-y-2 text-sm">
                {#each $cart.items as item}
                  <div class="flex justify-between">
                    <span>{item.name} × {item.quantity}</span>
                    <span>${item.subtotal.toFixed(2)}</span>
                  </div>
                {/each}
                <div class="border-t pt-2 font-semibold flex justify-between">
                  <span>Total:</span>
                  <span>${$cartTotal.toFixed(2)}</span>
                </div>
              </div>
            {/if}
          </div>

          <!-- Customer Information -->
          <div>
            <h3 class="font-semibold mb-3">Customer Information</h3>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <input
                type="text"
                placeholder="Full Name"
                bind:value={customerInfo.name}
                required
                disabled={processing}
                class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
              <input
                type="email"
                placeholder="Email"
                bind:value={customerInfo.email}
                required
                disabled={processing}
                class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
            </div>
          </div>

          <!-- Shipping Address -->
          <div>
            <h3 class="font-semibold mb-3">Shipping Address</h3>
            <div class="space-y-4">
              <input
                type="text"
                placeholder="Street Address"
                bind:value={customerInfo.address}
                required
                disabled={processing}
                class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
              <div class="grid grid-cols-2 gap-4">
                <input
                  type="text"
                  placeholder="City"
                  bind:value={customerInfo.city}
                  required
                  disabled={processing}
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                <input
                  type="text"
                  placeholder="ZIP Code"
                  bind:value={customerInfo.zipCode}
                  required
                  disabled={processing}
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
            </div>
          </div>

          <!-- Payment Information -->
          <div>
            <h3 class="font-semibold mb-3">Payment Information</h3>
            <div class="space-y-4">
              <input
                type="text"
                placeholder="Card Number"
                bind:value={customerInfo.cardNumber}
                on:input={(e) => customerInfo.cardNumber = formatCardNumber(e.currentTarget.value)}
                maxlength="19"
                required
                disabled={processing}
                class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
              <div class="grid grid-cols-2 gap-4">
                <input
                  type="text"
                  placeholder="MM/YY"
                  bind:value={customerInfo.expiryDate}
                  maxlength="5"
                  required
                  disabled={processing}
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                <input
                  type="text"
                  placeholder="CVV"
                  bind:value={customerInfo.cvv}
                  maxlength="4"
                  required
                  disabled={processing}
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
            </div>
          </div>

          <button
            type="submit"
            disabled={processing || !$cart || $cart.items.length === 0}
            class="w-full bg-purple-600 text-white py-3 px-4 rounded-lg hover:bg-purple-700 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
          >
            {#if processing}
              <span class="flex items-center justify-center">
                <svg class="animate-spin h-5 w-5 mr-3" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Processing...
              </span>
            {:else}
              Complete Order - ${$cartTotal.toFixed(2)}
            {/if}
          </button>
        </form>
      {/if}
    </div>
  </div>
{/if}