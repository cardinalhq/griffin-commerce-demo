// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// API service for backend communication

// Use relative URLs - they will be proxied through Vite dev server
const API_BASE = '';

export interface Product {
  id: string;
  name: string;
  description: string;
  price: number;
  stock: number;
  category?: string;
  image_url?: string;
}

export interface Cart {
  id: string;
  customer_id: string;
  items: CartItem[];
  total: number;
}

export interface CartItem {
  product_id: string;
  name: string;
  price: number;
  quantity: number;
  subtotal: number;
}

export interface PaymentRequest {
  order_id: string;
  amount: number;
  processor?: string;
}

export interface ShippingQuote {
  carrier: string;
  service: string;
  price: number;
  estimated_days: number;
}

class ApiService {
  // Product Catalog Service
  async getProducts(): Promise<Product[]> {
    const response = await fetch('/api/products');
    if (!response.ok) throw new Error('Failed to fetch products');
    const data = await response.json();
    return data.products || [];
  }

  async getProduct(id: string): Promise<Product> {
    const response = await fetch(`/api/products/${id}`);
    if (!response.ok) throw new Error('Product not found');
    return response.json();
  }

  // Cart Service
  async createCart(customerId: string): Promise<Cart> {
    const response = await fetch('/api/cart/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ customer_id: customerId })
    });
    if (!response.ok) throw new Error('Failed to create cart');
    return response.json();
  }

  async getCart(cartId: string): Promise<Cart> {
    const response = await fetch(`/api/cart/${cartId}`);
    if (!response.ok) throw new Error('Cart not found');
    return response.json();
  }

  async addToCart(cartId: string, productId: string, quantity: number): Promise<Cart> {
    const response = await fetch(`/api/cart/${cartId}/add`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product_id: productId, quantity })
    });
    if (!response.ok) throw new Error('Failed to add item to cart');
    return response.json();
  }

  async removeFromCart(cartId: string, productId: string): Promise<Cart> {
    const response = await fetch(`/api/cart/${cartId}/item/${productId}`, {
      method: 'DELETE'
    });
    if (!response.ok) throw new Error('Failed to remove item');
    return response.json();
  }

  async clearCart(cartId: string): Promise<Cart> {
    const response = await fetch(`/api/cart/${cartId}/clear`, {
      method: 'DELETE'
    });
    if (!response.ok) throw new Error('Failed to clear cart');
    return response.json();
  }

  // Payment Service
  async processPayment(request: PaymentRequest): Promise<any> {
    const response = await fetch('/api/payments/charge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request)
    });
    if (!response.ok) throw new Error('Payment failed');
    return response.json();
  }

  // Shipping Service
  async getShippingRates(zipCode: string): Promise<ShippingQuote[]> {
    const response = await fetch(`/api/shipping/rates?zip=${zipCode}`);
    if (!response.ok) throw new Error('Failed to get shipping rates');
    const data = await response.json();
    return data.quotes || [];
  }

  async getShippingQuote(weight: number, destination: string): Promise<ShippingQuote> {
    const response = await fetch('/api/shipping/quote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ weight, destination })
    });
    if (!response.ok) throw new Error('Failed to get shipping quote');
    return response.json();
  }

  // Recommendations Service
  async getRecommendations(count: number = 4): Promise<Product[]> {
    const response = await fetch(`/api/recommendations?count=${count}`);
    if (!response.ok) throw new Error('Failed to fetch recommendations');
    const data = await response.json();
    return data.products || [];
  }

  async getProductRecommendations(productId: string, count: number = 4): Promise<Product[]> {
    const response = await fetch(`/api/recommendations/product/${productId}?count=${count}`);
    if (!response.ok) throw new Error('Failed to fetch product recommendations');
    const data = await response.json();
    return data.products || [];
  }

  // Image Service
  async getProductImage(productId: string): Promise<string> {
    const response = await fetch(`/api/images/product/${productId}`);
    if (!response.ok) return null;
    const data = await response.json();
    return data.image_url;
  }

  // Complete checkout with payment and shipping
  async completeCheckout(cartId: string, paymentInfo: any, shippingInfo: any): Promise<any> {
    // Process payment first
    const paymentResult = await this.processPayment({
      order_id: cartId,
      amount: paymentInfo.amount,
      processor: paymentInfo.processor || 'PuppyPay'
    });

    // Create shipment
    const shipmentResult = await fetch('/api/shipping/ship', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        order_id: cartId,
        address: shippingInfo.address,
        carrier: shippingInfo.carrier
      })
    });

    return {
      payment: paymentResult,
      shipment: await shipmentResult.json(),
      order_id: cartId
    };
  }
}

export const api = new ApiService();