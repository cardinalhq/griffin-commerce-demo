<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2026 CardinalHQ, Inc. -->

<script lang="ts">
  import { cart, cartTotal } from '../stores/cart';
  import { fade } from 'svelte/transition';
  import { api } from '../api';
  import { onMount } from 'svelte';

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

  let selectedPaymentProcessor = 'PuppyPay';
  let selectedShippingCarrier = '';
  let shippingRates: any[] = [];
  let loadingShipping = false;
  let shippingCost = 0;

  let processing = false;
  let orderComplete = false;
  let orderDetails: any = null;
  let paymentError: string | null = null;

  const paymentProcessors = [
    { id: 'PuppyPay', name: 'PuppyPay', icon: '🐶', description: 'Fast and friendly' },
    { id: 'KittyCard', name: 'KittyCard', icon: '😺', description: 'Purr-fectly secure' },
    { id: 'DoggieCoin', name: 'DoggieCoin', icon: '🪙', description: 'Crypto for canines' }
  ];

  // Load shipping rates when zipcode is entered
  $: if (customerInfo.zipCode.length === 5 && shippingRates.length === 0) {
    loadShippingRates();
  }

  // Initialize with default rates on mount
  onMount(() => {
    if (shippingRates.length === 0) {
      shippingRates = [
        { carrier: 'PonyExpress', service: 'Standard', price: 5.99, estimated_days: 5 },
        { carrier: 'CatCarrier', service: 'Priority', price: 14.99, estimated_days: 2 },
        { carrier: 'AvianAir', service: 'Express', price: 24.99, estimated_days: 1 }
      ];
      selectedShippingCarrier = shippingRates[0].carrier;
      shippingCost = shippingRates[0].price;
    }
  });

  async function loadShippingRates() {
    loadingShipping = true;
    try {
      shippingRates = await api.getShippingRates(customerInfo.zipCode);
      if (shippingRates.length > 0) {
        selectedShippingCarrier = shippingRates[0].carrier;
        shippingCost = shippingRates[0].price;
      }
    } catch (error) {
      console.error('Failed to load shipping rates:', error);
      // Fallback rates
      shippingRates = [
        { carrier: 'PonyExpress', service: 'Standard', price: 5.99, estimated_days: 5 },
        { carrier: 'CatCarrier', service: 'Priority', price: 14.99, estimated_days: 2 },
        { carrier: 'AvianAir', service: 'Express', price: 24.99, estimated_days: 1 }
      ];
      selectedShippingCarrier = shippingRates[0].carrier;
      shippingCost = shippingRates[0].price;
    } finally {
      loadingShipping = false;
    }
  }

  async function handleCheckout() {
    processing = true;
    paymentError = null;

    try {
      const totalAmount = $cartTotal + shippingCost;

      // Complete checkout with payment and shipping
      const result = await api.completeCheckout(
        $cart?.id || '',
        {
          amount: totalAmount,
          processor: selectedPaymentProcessor,
          card_number: customerInfo.cardNumber,
          expiry: customerInfo.expiryDate,
          cvv: customerInfo.cvv
        },
        {
          address: `${customerInfo.address}, ${customerInfo.city} ${customerInfo.zipCode}`,
          carrier: selectedShippingCarrier,
          customer_name: customerInfo.name,
          customer_email: customerInfo.email
        }
      );

      orderDetails = {
        orderId: result.order_id,
        paymentId: result.payment.transaction_id,
        shipmentId: result.shipment.tracking_number,
        total: totalAmount,
        estimatedDelivery: result.shipment.estimated_delivery
      };

      orderComplete = true;

      // Clear cart after successful checkout
      setTimeout(() => {
        cart.reset();
        show = false;
        orderComplete = false;
        processing = false;
        resetForm();
      }, 5000);
    } catch (error: any) {
      console.error('Checkout failed:', error);
      paymentError = error.message || 'Payment failed. Please try a different payment method.';
      processing = false;
    }
  }

  function resetForm() {
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
    selectedPaymentProcessor = 'PuppyPay';
    shippingRates = [];
    shippingCost = 0;
  }

  function formatCardNumber(value: string) {
    const v = value.replace(/\s+/g, '').replace(/[^0-9]/gi, '');
    const matches = v.match(/\d{4,16}/g);
    const match = (matches && matches[0]) || '';
    const parts = [];

    for (let i = 0, len = match.length; i < len; i += 4) {
      parts.push(match.substring(i, i + 4));
    }

    return parts.length ? parts.join(' ') : value;
  }

  function updateShippingCost(carrier: string) {
    const rate = shippingRates.find(r => r.carrier === carrier);
    if (rate) {
      shippingCost = rate.price;
    }
  }

  $: grandTotal = $cartTotal + shippingCost;
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
    <div class="bg-white rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
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

          <div class="bg-gray-50 rounded-lg p-4 text-left max-w-md mx-auto">
            <p class="text-sm text-gray-600 mb-2">
              <span class="font-semibold">Order ID:</span> {orderDetails.orderId}
            </p>
            <p class="text-sm text-gray-600 mb-2">
              <span class="font-semibold">Payment ID:</span> {orderDetails.paymentId}
            </p>
            <p class="text-sm text-gray-600 mb-2">
              <span class="font-semibold">Tracking #:</span> {orderDetails.shipmentId}
            </p>
            <p class="text-sm text-gray-600 mb-2">
              <span class="font-semibold">Total Paid:</span> ${orderDetails.total.toFixed(2)}
            </p>
            <p class="text-sm text-gray-600">
              <span class="font-semibold">Est. Delivery:</span> {orderDetails.estimatedDelivery || '3-5 business days'}
            </p>
          </div>
        </div>
      {:else}
        <div class="p-6 border-b">
          <div class="flex items-center justify-between">
            <h2 class="text-2xl font-bold">Enhanced Checkout</h2>
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
                <div class="border-t pt-2">
                  <div class="flex justify-between">
                    <span>Subtotal:</span>
                    <span>${$cartTotal.toFixed(2)}</span>
                  </div>
                  <div class="flex justify-between">
                    <span>Shipping:</span>
                    <span>${shippingCost.toFixed(2)}</span>
                  </div>
                </div>
                <div class="border-t pt-2 font-semibold flex justify-between text-base">
                  <span>Total:</span>
                  <span>${grandTotal.toFixed(2)}</span>
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
                  maxlength="5"
                  disabled={processing}
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
            </div>
          </div>

          <!-- Shipping Options -->
          {#if shippingRates.length > 0}
            <div>
              <h3 class="font-semibold mb-3">Shipping Method</h3>
              {#if loadingShipping}
                <div class="text-center py-4">
                  <div class="animate-spin inline-block h-6 w-6 border-2 border-purple-600 rounded-full border-t-transparent"></div>
                </div>
              {:else}
                <div class="space-y-2">
                  {#each shippingRates as rate}
                    <label class="flex items-center p-3 border rounded-lg cursor-pointer hover:bg-gray-50 {selectedShippingCarrier === rate.carrier ? 'border-purple-600 bg-purple-50' : 'border-gray-300'}">
                      <input
                        type="radio"
                        name="shipping"
                        value={rate.carrier}
                        bind:group={selectedShippingCarrier}
                        on:change={() => updateShippingCost(rate.carrier)}
                        class="mr-3"
                        disabled={processing}
                      />
                      <div class="flex-1">
                        <div class="font-medium">{rate.carrier} - {rate.service}</div>
                        <div class="text-sm text-gray-600">Delivery in {rate.estimated_days} day(s)</div>
                      </div>
                      <div class="font-semibold">${rate.price.toFixed(2)}</div>
                    </label>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}

          <!-- Payment Method -->
          <div>
            <h3 class="font-semibold mb-3">Payment Method</h3>
            <div class="grid grid-cols-3 gap-3 mb-4">
              {#each paymentProcessors as processor}
                <label class="flex flex-col items-center p-3 border rounded-lg cursor-pointer hover:bg-gray-50 {selectedPaymentProcessor === processor.id ? 'border-purple-600 bg-purple-50' : 'border-gray-300'}">
                  <input
                    type="radio"
                    name="processor"
                    value={processor.id}
                    bind:group={selectedPaymentProcessor}
                    class="sr-only"
                    disabled={processing}
                  />
                  <span class="text-2xl mb-1">{processor.icon}</span>
                  <span class="font-medium text-sm">{processor.name}</span>
                  <span class="text-xs text-gray-500">{processor.description}</span>
                </label>
              {/each}
            </div>

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

          {#if paymentError}
            <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
              {paymentError}
            </div>
          {/if}

          <button
            type="submit"
            disabled={processing || !$cart || $cart.items.length === 0 || !selectedShippingCarrier}
            class="w-full bg-purple-600 text-white py-3 px-4 rounded-lg hover:bg-purple-700 transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
          >
            {#if processing}
              <span class="flex items-center justify-center">
                <svg class="animate-spin h-5 w-5 mr-3" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Processing Payment...
              </span>
            {:else}
              Pay ${grandTotal.toFixed(2)} with {selectedPaymentProcessor}
            {/if}
          </button>
        </form>
      {/if}
    </div>
  </div>
{/if}