# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 CardinalHQ, Inc.

import random
import time
import uuid
from locust import HttpUser, task, between, events
from locust.env import Environment
import logging

logger = logging.getLogger(__name__)

class EcommerceUser(HttpUser):
    """Simulates a regular user browsing through the frontend"""
    wait_time = between(1, 3)  # Think time between requests

    # All requests go through the frontend proxy
    host = "http://localhost:5173"  # Frontend dev server

    def on_start(self):
        """Called when a user starts - initialize session"""
        self.customer_id = str(uuid.uuid4())
        self.cart_id = None
        self.products = []
        self.selected_product = None

        # Load the main page first (like a real user)
        self.client.get("/", name="Load Homepage")

        # Create a cart for this user session
        self.create_cart()

        # Load product catalog
        self.load_catalog()

    def create_cart(self):
        """Create a shopping cart for this session"""
        with self.client.post(
            "/api/cart/create",
            json={"customer_id": self.customer_id},
            catch_response=True,
            name="Create Cart"
        ) as response:
            if response.status_code == 200:
                data = response.json()
                self.cart_id = data.get("id")
                response.success()
            else:
                response.failure(f"Failed to create cart: {response.status_code}")

    def load_catalog(self):
        """Load the product catalog"""
        with self.client.get("/api/products", catch_response=True, name="Get Products") as response:
            if response.status_code == 200:
                self.products = response.json()
                response.success()
            else:
                response.failure(f"Failed to load catalog: {response.status_code}")

    @task(30)
    def browse_catalog(self):
        """Browse the product catalog"""
        self.client.get("/api/products", name="Browse Catalog")

        # Simulate scrolling/pagination delay
        time.sleep(random.uniform(0.5, 2))

    @task(25)
    def view_product_details(self):
        """View details of a specific product"""
        if not self.products:
            self.load_catalog()

        if self.products:
            product = random.choice(self.products)
            self.selected_product = product

            # Get product details
            self.client.get(
                f"/api/products/{product['id']}",
                name="View Product Details"
            )

            # Load product image through static path (as frontend does)
            with self.client.get(
                f"/static/products/{product['id']}.jpg",
                catch_response=True,
                name="Load Product Image"
            ) as response:
                if response.status_code != 200:
                    # Try alternative image endpoint
                    self.client.get(
                        f"/api/images/product/{product['id']}",
                        name="Load Product Image (API)"
                    )
                else:
                    response.success()

            # Get recommendations for this product
            self.client.get(
                f"/api/recommendations/product/{product['id']}",
                name="Get Product Recommendations"
            )

    @task(20)
    def add_to_cart(self):
        """Add a product to the shopping cart"""
        if not self.cart_id:
            self.create_cart()

        if not self.products:
            self.load_catalog()

        if self.products and self.cart_id:
            # Select a product to add
            product = self.selected_product or random.choice(self.products)
            quantity = random.randint(1, 3)

            with self.client.post(
                f"/api/cart/{self.cart_id}/add",
                json={
                    "product_id": product["id"],
                    "quantity": quantity
                },
                catch_response=True,
                name="Add to Cart"
            ) as response:
                if response.status_code == 200:
                    response.success()
                    logger.info(f"Added {quantity} x {product['name']} to cart")
                else:
                    response.failure(f"Failed to add to cart: {response.status_code}")

    @task(15)
    def view_cart(self):
        """View the shopping cart"""
        if self.cart_id:
            self.client.get(
                f"/api/cart/{self.cart_id}",
                name="View Cart"
            )

    @task(5)
    def remove_from_cart(self):
        """Remove an item from the cart"""
        if self.cart_id and self.products:
            # First, view the cart to see what's in it
            with self.client.get(
                f"/api/cart/{self.cart_id}",
                catch_response=True,
                name="View Cart (before remove)"
            ) as response:
                if response.status_code == 200:
                    cart_data = response.json()
                    items = cart_data.get("items", [])
                    if items:
                        # Remove a random item
                        item_to_remove = random.choice(items)
                        self.client.delete(
                            f"/api/cart/{self.cart_id}/item/{item_to_remove['product_id']}",
                            name="Remove from Cart"
                        )
                    response.success()
                else:
                    response.failure(f"Failed to view cart: {response.status_code}")

    @task(3)
    def checkout(self):
        """Complete checkout process"""
        if not self.cart_id:
            return

        # View cart before checkout
        with self.client.get(
            f"/api/cart/{self.cart_id}",
            catch_response=True,
            name="View Cart (checkout)"
        ) as response:
            if response.status_code != 200:
                response.failure("Cart not found")
                return

            cart_data = response.json()
            if not cart_data.get("items"):
                response.success()
                return  # Cart is empty

            response.success()

        # Initiate checkout
        with self.client.post(
            f"/api/cart/{self.cart_id}/checkout",
            json={
                "payment_method": "credit_card",
                "shipping_address": {
                    "street": "123 Test St",
                    "city": "Test City",
                    "state": "TS",
                    "zip": "12345"
                }
            },
            catch_response=True,
            name="Checkout"
        ) as response:
            if response.status_code == 200:
                response.success()
                logger.info(f"Checkout completed for cart {self.cart_id}")
                # Create a new cart for next shopping session
                self.create_cart()
            else:
                response.failure(f"Checkout failed: {response.status_code}")

    @task(2)
    def check_recommendations(self):
        """Check general recommendations"""
        self.client.get("/api/recommendations", name="Get Recommendations")


class MobileUser(EcommerceUser):
    """Mobile users with slower interactions and different behavior patterns"""
    wait_time = between(2, 5)  # Slower interactions on mobile

    @task(40)  # Mobile users browse more
    def browse_catalog(self):
        super().browse_catalog()

    @task(20)  # Less likely to add to cart on mobile
    def add_to_cart(self):
        super().add_to_cart()

    @task(5)  # Much less likely to checkout on mobile
    def checkout(self):
        super().checkout()


class PowerUser(EcommerceUser):
    """Power users who shop more aggressively"""
    wait_time = between(0.5, 1.5)  # Faster interactions

    @task(10)  # Less browsing
    def browse_catalog(self):
        super().browse_catalog()

    @task(35)  # More adding to cart
    def add_to_cart(self):
        super().add_to_cart()

    @task(15)  # More likely to checkout
    def checkout(self):
        super().checkout()


# Event handlers for reporting
@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    print("Load test starting...")

@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    print("Load test stopping...")
