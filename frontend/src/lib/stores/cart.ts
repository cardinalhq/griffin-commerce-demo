import { writable, derived } from 'svelte/store';
import { api, type Cart, type CartItem } from '../api';

function createCartStore() {
  const { subscribe, set, update } = writable<Cart | null>(null);

  let cartId: string | null = localStorage.getItem('cartId');
  const customerId = localStorage.getItem('customerId') || `customer-${Date.now()}`;

  if (!localStorage.getItem('customerId')) {
    localStorage.setItem('customerId', customerId);
  }

  return {
    subscribe,

    async init() {
      if (cartId) {
        try {
          const cart = await api.getCart(cartId);
          set(cart);
        } catch {
          // Cart not found, create new one
          await this.createCart();
        }
      } else {
        await this.createCart();
      }
    },

    async createCart() {
      try {
        const cart = await api.createCart(customerId);
        cartId = cart.id;
        localStorage.setItem('cartId', cartId);
        set(cart);
      } catch (error) {
        console.error('Failed to create cart:', error);
      }
    },

    async addItem(productId: string, quantity: number = 1) {
      if (!cartId) await this.createCart();
      if (!cartId) return;

      try {
        const cart = await api.addToCart(cartId, productId, quantity);
        set(cart);
      } catch (error) {
        console.error('Failed to add item to cart:', error);
        throw error;
      }
    },

    async removeItem(productId: string) {
      if (!cartId) return;

      try {
        const cart = await api.removeFromCart(cartId, productId);
        set(cart);
      } catch (error) {
        console.error('Failed to remove item from cart:', error);
        throw error;
      }
    },

    async clear() {
      if (!cartId) return;

      try {
        const cart = await api.clearCart(cartId);
        set(cart);
      } catch (error) {
        console.error('Failed to clear cart:', error);
        throw error;
      }
    },

    reset() {
      set(null);
      cartId = null;
      localStorage.removeItem('cartId');
    }
  };
}

export const cart = createCartStore();

// Derived store for cart count
export const cartCount = derived(
  cart,
  $cart => $cart ? $cart.items.reduce((acc, item) => acc + item.quantity, 0) : 0
);

// Derived store for cart total
export const cartTotal = derived(
  cart,
  $cart => $cart ? $cart.total : 0
);