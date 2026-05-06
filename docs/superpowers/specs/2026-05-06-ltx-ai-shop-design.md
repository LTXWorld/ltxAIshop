# ltxAI Shop MVP Design

Date: 2026-05-06

## Goal

Build an AI product sales website for `ltxAIshop.bfsmlt.com`, deployed on VPS `154.64.230.197`. The MVP sells several classes of legally authorized digital products. Initial products are labeled A, B, and C, with future expansion to D, E, F, and other product types.

The site supports customer registration and login, product browsing, shopping cart checkout, Alipay payment, automatic or manual fulfillment based on product strategy, and user order history. It also includes an administrator backend for managing products, inventory, orders, users, and fulfillment.

## Architecture

Use a Go backend API and a React/Vite frontend.

The Go service owns all trusted business logic:

- Email and password authentication
- Customer and administrator authorization
- Product, cart, order, payment, inventory, and delivery APIs
- Alipay order creation, notification verification, and payment reconciliation
- Automatic fulfillment for eligible digital products

The React/Vite frontend provides:

- Public storefront
- Product list and product detail pages
- Cart and checkout pages
- Customer login, registration, account, and order pages
- Administrator backend UI

The built frontend is served by Nginx. Nginx also terminates HTTPS and reverse proxies `/api/*` to the Go service.

## Deployment

Use Docker Compose on the VPS with these main services:

- `nginx`: HTTPS, static frontend hosting, API reverse proxy
- `api`: Go backend service
- `postgres`: PostgreSQL database

The domain `ltxAIshop.bfsmlt.com` points to VPS `154.64.230.197`. GitHub Actions will later run tests, build artifacts or images, and deploy to the VPS over SSH by pulling the latest code or images and running `docker compose up -d`.

Runtime configuration is injected through environment variables. Alipay settings include:

- `ALIPAY_APP_ID`
- Alipay gateway URL
- Application private key
- Alipay public key or certificate settings
- `ALIPAY_NOTIFY_URL`
- `ALIPAY_RETURN_URL`

The system should support sandbox settings during development and production settings after the Alipay Open Platform application is approved.

## Payment Flow

Use Alipay Open Platform's PC website payment flow, targeting `alipay.trade.page.pay`.

The checkout flow is:

1. Customer submits checkout.
2. Backend creates a local order with status `pending_payment`.
3. Backend creates an Alipay payment request and returns the payment form or redirect target.
4. Customer pays on the Alipay cashier page.
5. Alipay redirects the browser to `return_url`.
6. Alipay sends an asynchronous server notification to `notify_url`.
7. Backend verifies the notification signature and validates `app_id`, merchant order number, amount, and trade status.
8. Backend marks the order as paid and triggers fulfillment.

The browser return page must not be treated as proof of payment. It shows the current order state and polls the backend. If the asynchronous notification is delayed, the backend may query Alipay for the order status before showing a final result.

Payment processing must be idempotent. Repeated Alipay notifications for the same order must not create duplicate delivery records or allocate inventory more than once.

## Order States

Initial order states:

- `pending_payment`: order created, waiting for payment
- `paid`: payment verified
- `fulfilled`: automatic fulfillment completed
- `manual_required`: paid order needs manual handling
- `cancelled`: order cancelled before successful payment
- `payment_failed`: payment failed or was rejected
- `refunded`: refund recorded

For products fulfilled automatically, a paid order transitions to `fulfilled` after delivery succeeds. For products that require contact through WeChat or another manual channel, a paid order transitions to `manual_required`.

## Products And Fulfillment Strategies

Products are not hardcoded as A, B, or C in the core order flow. Each product has a fulfillment strategy.

Initial strategies:

- `digital_credentials`: payment allocates one inventory item and shows account credentials to the customer. This covers initial A/C-like products.
- `manual_contact`: payment marks the order as requiring manual handling and shows contact instructions. This covers initial B-like products.
- `digital_code`: reserved for future card codes, license keys, API keys, or similar digital goods.

Future D/E/F product types should usually be added by creating a new strategy, adding strategy-specific admin forms, and implementing a fulfillment handler. The payment and order state machines should remain unchanged.

## Inventory

Administrators can create inventory entries for products that use inventory-backed fulfillment.

For `digital_credentials`, each inventory entry may include:

- Account
- Password
- Auxiliary email
- Auxiliary email password, if needed
- 2FA helper code or recovery code
- Notes

The inventory model should include a flexible `payload JSONB` field so future strategies can store strategy-specific delivery data without schema churn. The MVP may also expose fixed fields in the admin UI for the A/C credential workflow.

Inventory states:

- `available`
- `reserved`, optional if checkout reservation is later added
- `sold`
- `disabled`

On verified payment, the backend uses a PostgreSQL transaction and row-level locking to select one available inventory item, mark it sold, bind it to the order item, and create a delivery record. This prevents the same account or code from being sold twice under concurrent payment notifications.

If stock is unavailable after payment, the order should be flagged for administrator attention rather than silently failing. The customer should see a clear pending-delivery state and contact instructions.

## Customer Features

Customers can:

- Register with email and password
- Log in and log out
- Browse products
- View product detail pages
- Add products to cart
- Checkout and pay with Alipay
- View order list and order detail pages
- View delivered content for paid and fulfilled orders
- See contact instructions for manual orders

Password reset is supported when SMTP is configured. If SMTP is not configured, password reset can be hidden or replaced with administrator contact instructions.

## Administrator Features

The MVP supports one administrator role. The schema should leave room for multiple administrators later through either a role field on users or a separate admin user model.

Administrator functions:

- Manage products, prices, descriptions, images, publish state, and fulfillment strategy
- Manage inventory entries and inspect inventory states
- View and search orders
- View users
- Manually update order status when needed
- View manual fulfillment orders
- Configure or edit contact instructions for manual strategies

Future hardening can add administrator 2FA, IP restrictions, operation audit logs, and finer-grained roles.

## Security

Passwords must be stored with a strong password hashing algorithm.

MVP delivery fields are stored in plaintext as an explicit speed and simplicity tradeoff. Access must still be strictly limited:

- Customers can only access their own orders and delivery records.
- Customers can only view delivery content after payment is verified.
- Administrators must be authenticated to access products, inventory, users, orders, and deliveries.
- Alipay notifications must pass signature verification and business field validation before changing order state.

The delivery and inventory code should be centralized so plaintext storage can later be replaced with field encryption without rewriting checkout, order, or payment flows.

Production deployment must use HTTPS.

## Data Model

Core tables:

- `users`: customer accounts and possibly role flags
- `admin_users`, optional if administrator accounts are separated
- `products`: product metadata, price, status, and fulfillment strategy
- `product_images`: product image metadata
- `inventory_items`: inventory state, product link, sold order item link, fixed display fields, and `payload JSONB`
- `carts`: active customer carts
- `cart_items`: products and quantities in carts
- `orders`: customer, total amount, status, timestamps
- `order_items`: purchased products, price snapshots, fulfillment strategy snapshots
- `payments`: Alipay merchant order number, trade number, amount, status, notification metadata
- `deliveries`: order item delivery state, linked inventory item, visible delivery payload

Prices and product names should be snapshotted onto `order_items` so historical orders remain accurate after products change.

## Testing

High-risk backend paths need automated tests:

- Registration and login
- Customer authorization
- Administrator authorization
- Product and inventory management permissions
- Order creation
- Alipay notification signature handling
- Rejecting mismatched order amount, app ID, or trade status
- Repeated notification idempotency
- Automatic inventory allocation
- Concurrent inventory allocation
- Out-of-stock after payment
- Manual-contact fulfillment
- Customer access limited to own orders

Frontend tests should cover the main user flows:

- Register or login
- Browse product
- Add to cart
- Checkout redirect start
- Payment return status page
- Order list and order detail
- Admin product, inventory, and order screens

## Rollout

1. Scaffold the Go API, React/Vite frontend, Docker Compose, and PostgreSQL migrations.
2. Implement authentication and authorization.
3. Implement product, cart, order, inventory, delivery, and admin APIs.
4. Implement Alipay sandbox payment creation, notify verification, return page, polling, and query fallback.
5. Build the storefront, customer center, and admin UI.
6. Run local Docker Compose end-to-end.
7. Configure domain DNS, HTTPS, and VPS deployment.
8. Add GitHub Actions for test, build, and deploy.
9. Switch from Alipay sandbox to production settings after the merchant application is approved.
10. Validate with a low-value production transaction before normal sales.

## Non-Goals For MVP

- Multi-admin role permissions
- Field-level encryption for delivered credentials
- Automated refunds
- Complex promotion or coupon systems
- Multi-currency payments
- Product recommendation engine
- Email marketing
