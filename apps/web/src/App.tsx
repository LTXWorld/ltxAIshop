import {
  Boxes,
  CreditCard,
  LogIn,
  LogOut,
  PackagePlus,
  RefreshCw,
  ShoppingCart,
  UserRound
} from "lucide-react";
import { FormEvent, useEffect, useMemo, useState } from "react";
import "./styles.css";

type View = "shop" | "cart" | "orders" | "account" | "admin";
type AuthMode = "login" | "register";

type User = {
  id: number;
  email: string;
  role: "customer" | "admin";
};

type Product = {
  id: number;
  name: string;
  slug: string;
  description: string;
  priceCents: number;
  currency: string;
  status: string;
  fulfillmentStrategy: string;
  imageUrl: string;
};

type CartItem = {
  productId: number;
  productName: string;
  productSlug: string;
  priceCents: number;
  currency: string;
  fulfillmentStrategy: string;
  quantity: number;
  lineTotalCents: number;
};

type Cart = {
  items: CartItem[];
  totalCents: number;
  currency: string;
  itemCount: number;
};

type Order = {
  id: number;
  totalCents: number;
  currency: string;
  status: string;
  items: CartItem[];
};

type Payment = {
  id: number;
  orderId: number;
  provider: string;
  amountCents: number;
  currency: string;
  status: string;
  paymentForm?: string;
};

const emptyCart: Cart = { items: [], totalCents: 0, currency: "CNY", itemCount: 0 };

export default function App() {
  const [view, setView] = useState<View>("shop");
  const [token, setToken] = useState(() => localStorage.getItem("ltxai_token") || "");
  const [user, setUser] = useState<User | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [cart, setCart] = useState<Cart>(emptyCart);
  const [orders, setOrders] = useState<Order[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);

  const cartSummary = useMemo(() => {
    if (cart.itemCount === 0) return "Cart empty";
    return `${cart.itemCount} item${cart.itemCount === 1 ? "" : "s"} · ${money(cart.totalCents, cart.currency)}`;
  }, [cart]);

  useEffect(() => {
    void refreshProducts();
  }, []);

  useEffect(() => {
    if (!token) {
      setUser(null);
      setCart(emptyCart);
      setOrders([]);
      return;
    }
    localStorage.setItem("ltxai_token", token);
    void refreshSession(token);
  }, [token]);

  async function refreshSession(authToken = token) {
    try {
      const me = await api<User>("/api/me", { token: authToken });
      setUser(me);
      await Promise.all([refreshCart(authToken), refreshOrders(authToken)]);
    } catch {
      localStorage.removeItem("ltxai_token");
      setToken("");
    }
  }

  async function refreshProducts() {
    const data = await api<{ products: Product[] }>("/api/products");
    setProducts(data.products);
  }

  async function refreshCart(authToken = token) {
    if (!authToken) return;
    const data = await api<Cart>("/api/cart", { token: authToken });
    setCart(data);
  }

  async function refreshOrders(authToken = token) {
    if (!authToken) return;
    const data = await api<{ orders: Order[] }>("/api/orders", { token: authToken });
    setOrders(data.orders);
  }

  async function addToCart(product: Product) {
    if (!token) {
      setView("account");
      setNotice("Sign in before adding products.");
      return;
    }
    setLoading(true);
    try {
      const existing = cart.items.find((item) => item.productId === product.id);
      const data = await api<Cart>("/api/cart/items", {
        method: "PUT",
        token,
        body: { productId: product.id, quantity: (existing?.quantity || 0) + 1 }
      });
      setCart(data);
      setNotice(`${product.name} added to cart.`);
      setView("cart");
    } finally {
      setLoading(false);
    }
  }

  async function setQuantity(productId: number, quantity: number) {
    const data = await api<Cart>("/api/cart/items", {
      method: "PUT",
      token,
      body: { productId, quantity }
    });
    setCart(data);
  }

  async function removeItem(productId: number) {
    const data = await api<Cart>(`/api/cart/items/${productId}`, { method: "DELETE", token });
    setCart(data);
  }

  async function createOrder() {
    setLoading(true);
    try {
      const order = await api<Order>("/api/orders", { method: "POST", token });
      setCart(emptyCart);
      setSelectedOrder(order);
      await refreshOrders();
      setView("orders");
      setNotice(`Order #${order.id} is waiting for payment.`);
    } finally {
      setLoading(false);
    }
  }

  async function startPayment(order: Order) {
    setLoading(true);
    try {
      let created: Payment;
      try {
        created = await api<Payment>("/api/payments", {
          method: "POST",
          token,
          body: { orderId: order.id, provider: "alipay" }
        });
      } catch {
        created = await api<Payment>("/api/payments", {
          method: "POST",
          token,
          body: { orderId: order.id, provider: "manual" }
        });
      }

      setPayment(created);
      if (created.paymentForm) {
        submitPaymentForm(created.paymentForm);
        return;
      }

      const confirmed = await api<Payment>(`/api/payments/${created.id}/confirm`, {
        method: "POST",
        token,
        body: {
          amountCents: created.amountCents,
          providerTradeNo: `manual-web-${Date.now()}`,
          rawPayload: { source: "web-fallback" }
        }
      });
      setPayment(confirmed);
      await refreshOrders();
      setNotice(`Order #${order.id} is paid.`);
    } finally {
      setLoading(false);
    }
  }

  function logout() {
    localStorage.removeItem("ltxai_token");
    setToken("");
    setView("shop");
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <button className="brand" onClick={() => setView("shop")} type="button">
          <ShoppingCart aria-hidden="true" />
          <span>ltxAI Shop</span>
        </button>
        <nav className="nav-tabs" aria-label="Main navigation">
          <button className={view === "shop" ? "active" : ""} onClick={() => setView("shop")} type="button">
            Products
          </button>
          <button className={view === "cart" ? "active" : ""} onClick={() => setView("cart")} type="button">
            Cart
          </button>
          <button className={view === "orders" ? "active" : ""} onClick={() => setView("orders")} type="button">
            Orders
          </button>
          <button className={view === "account" ? "active" : ""} onClick={() => setView("account")} type="button">
            Account
          </button>
          {user?.role === "admin" ? (
            <button className={view === "admin" ? "active" : ""} onClick={() => setView("admin")} type="button">
              Admin
            </button>
          ) : null}
        </nav>
        <div className="session-chip">
          <span>{user ? user.email : "Guest"}</span>
          {user ? (
            <button aria-label="Log out" className="icon-button" onClick={logout} type="button">
              <LogOut aria-hidden="true" />
            </button>
          ) : (
            <button aria-label="Sign in" className="icon-button" onClick={() => setView("account")} type="button">
              <LogIn aria-hidden="true" />
            </button>
          )}
        </div>
      </header>

      {notice ? <div className="notice">{notice}</div> : null}

      <section className="workspace">
        <aside className="side-panel">
          <div>
            <p className="eyebrow">Session</p>
            <h1>AI product marketplace</h1>
          </div>
          <div className="metric">
            <ShoppingCart aria-hidden="true" />
            <span>{cartSummary}</span>
          </div>
          <div className="metric">
            <Boxes aria-hidden="true" />
            <span>{products.length} published products</span>
          </div>
          <button className="secondary wide" onClick={() => void refreshProducts()} type="button">
            <RefreshCw aria-hidden="true" />
            Refresh
          </button>
        </aside>

        <div className="content-panel">
          {view === "shop" ? <Shop products={products} loading={loading} onAdd={addToCart} /> : null}
          {view === "cart" ? (
            <CartView cart={cart} loading={loading} onCheckout={createOrder} onRemove={removeItem} onSetQuantity={setQuantity} />
          ) : null}
          {view === "orders" ? (
            <OrdersView
              loading={loading}
              orders={orders}
              payment={payment}
              selectedOrder={selectedOrder}
              onPay={startPayment}
              onSelect={setSelectedOrder}
            />
          ) : null}
          {view === "account" ? (
            <AccountView
              user={user}
              onAuthenticated={(nextToken) => {
                setToken(nextToken);
                setNotice("Signed in.");
                setView("shop");
              }}
            />
          ) : null}
          {view === "admin" && user?.role === "admin" ? (
            <AdminView token={token} onChanged={() => void refreshProducts()} />
          ) : null}
        </div>
      </section>
    </main>
  );
}

function Shop({ products, loading, onAdd }: { products: Product[]; loading: boolean; onAdd: (product: Product) => Promise<void> }) {
  return (
    <section>
      <PanelTitle icon={<Boxes aria-hidden="true" />} title="Products" />
      <div className="product-grid">
        {products.map((product) => (
          <article className="product-card" key={product.id}>
            <div className="product-media">{product.imageUrl ? <img alt="" src={product.imageUrl} /> : <PackagePlus aria-hidden="true" />}</div>
            <div>
              <h2>{product.name}</h2>
              <p>{product.description || fulfillmentLabel(product.fulfillmentStrategy)}</p>
            </div>
            <div className="card-footer">
              <strong>{money(product.priceCents, product.currency)}</strong>
              <button disabled={loading} onClick={() => void onAdd(product)} type="button">
                <ShoppingCart aria-hidden="true" />
                Add
              </button>
            </div>
          </article>
        ))}
        {products.length === 0 ? <EmptyState text="No published products yet." /> : null}
      </div>
    </section>
  );
}

function CartView({
  cart,
  loading,
  onCheckout,
  onRemove,
  onSetQuantity
}: {
  cart: Cart;
  loading: boolean;
  onCheckout: () => Promise<void>;
  onRemove: (productId: number) => Promise<void>;
  onSetQuantity: (productId: number, quantity: number) => Promise<void>;
}) {
  return (
    <section>
      <PanelTitle icon={<ShoppingCart aria-hidden="true" />} title="Cart" />
      <div className="table-list">
        {cart.items.map((item) => (
          <div className="line-item" key={item.productId}>
            <div>
              <strong>{item.productName}</strong>
              <span>{money(item.priceCents, item.currency)}</span>
            </div>
            <input
              aria-label={`Quantity for ${item.productName}`}
              min={1}
              onChange={(event) => void onSetQuantity(item.productId, Number(event.target.value))}
              type="number"
              value={item.quantity}
            />
            <strong>{money(item.lineTotalCents, item.currency)}</strong>
            <button className="secondary" onClick={() => void onRemove(item.productId)} type="button">
              Remove
            </button>
          </div>
        ))}
      </div>
      {cart.items.length === 0 ? <EmptyState text="Your cart is empty." /> : null}
      <div className="summary-bar">
        <strong>Total {money(cart.totalCents, cart.currency || "CNY")}</strong>
        <button disabled={loading || cart.items.length === 0} onClick={() => void onCheckout()} type="button">
          Create order
        </button>
      </div>
    </section>
  );
}

function OrdersView({
  loading,
  orders,
  payment,
  selectedOrder,
  onPay,
  onSelect
}: {
  loading: boolean;
  orders: Order[];
  payment: Payment | null;
  selectedOrder: Order | null;
  onPay: (order: Order) => Promise<void>;
  onSelect: (order: Order) => void;
}) {
  const active = selectedOrder || orders[0] || null;
  return (
    <section>
      <PanelTitle icon={<CreditCard aria-hidden="true" />} title="Orders" />
      <div className="orders-layout">
        <div className="table-list">
          {orders.map((order) => (
            <button className="order-row" key={order.id} onClick={() => onSelect(order)} type="button">
              <span>#{order.id}</span>
              <strong>{money(order.totalCents, order.currency)}</strong>
              <span>{order.status}</span>
            </button>
          ))}
          {orders.length === 0 ? <EmptyState text="No orders yet." /> : null}
        </div>
        <div className="detail-panel">
          {active ? (
            <>
              <h2>Order #{active.id}</h2>
              <p>{active.status}</p>
              {active.items.map((item) => (
                <div className="compact-row" key={item.productId}>
                  <span>{item.productName}</span>
                  <strong>{item.quantity} x {money(item.priceCents, item.currency)}</strong>
                </div>
              ))}
              <div className="summary-bar inline">
                <strong>{money(active.totalCents, active.currency)}</strong>
                <button disabled={loading || active.status !== "pending_payment"} onClick={() => void onPay(active)} type="button">
                  Pay
                </button>
              </div>
              {payment?.orderId === active.id ? <p className="fine-print">Payment {payment.status}</p> : null}
            </>
          ) : (
            <EmptyState text="Select an order." />
          )}
        </div>
      </div>
    </section>
  );
}

function AccountView({ user, onAuthenticated }: { user: User | null; onAuthenticated: (token: string) => void }) {
  const [mode, setMode] = useState<AuthMode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const data = await api<{ token: string; user: User }>(mode === "login" ? "/api/auth/login" : "/api/auth/register", {
        method: "POST",
        body: { email, password }
      });
      onAuthenticated(data.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed");
    }
  }

  if (user) {
    return (
      <section>
        <PanelTitle icon={<UserRound aria-hidden="true" />} title="Account" />
        <div className="detail-panel">
          <h2>{user.email}</h2>
          <p>{user.role}</p>
        </div>
      </section>
    );
  }

  return (
    <section>
      <PanelTitle icon={<UserRound aria-hidden="true" />} title="Account" />
      <div className="segmented">
        <button className={mode === "login" ? "active" : ""} onClick={() => setMode("login")} type="button">Login</button>
        <button className={mode === "register" ? "active" : ""} onClick={() => setMode("register")} type="button">Register</button>
      </div>
      <form className="form-stack" onSubmit={(event) => void submit(event)}>
        <label>
          Email
          <input autoComplete="email" onChange={(event) => setEmail(event.target.value)} type="email" value={email} />
        </label>
        <label>
          Password
          <input autoComplete={mode === "login" ? "current-password" : "new-password"} onChange={(event) => setPassword(event.target.value)} type="password" value={password} />
        </label>
        {error ? <p className="form-error">{error}</p> : null}
        <button type="submit">{mode === "login" ? "Login" : "Register"}</button>
      </form>
    </section>
  );
}

function AdminView({ token, onChanged }: { token: string; onChanged: () => void }) {
  const [form, setForm] = useState({
    name: "",
    slug: "",
    description: "",
    priceCents: 9900,
    status: "published",
    fulfillmentStrategy: "manual_contact"
  });
  const [message, setMessage] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    await api<Product>("/api/admin/products", { method: "POST", token, body: { ...form, currency: "CNY", imageUrl: "" } });
    setMessage("Product created.");
    onChanged();
  }

  return (
    <section>
      <PanelTitle icon={<PackagePlus aria-hidden="true" />} title="Admin products" />
      <form className="form-grid" onSubmit={(event) => void submit(event)}>
        <label>
          Name
          <input onChange={(event) => setForm({ ...form, name: event.target.value })} value={form.name} />
        </label>
        <label>
          Slug
          <input onChange={(event) => setForm({ ...form, slug: event.target.value })} value={form.slug} />
        </label>
        <label>
          Price cents
          <input onChange={(event) => setForm({ ...form, priceCents: Number(event.target.value) })} type="number" value={form.priceCents} />
        </label>
        <label>
          Fulfillment
          <select onChange={(event) => setForm({ ...form, fulfillmentStrategy: event.target.value })} value={form.fulfillmentStrategy}>
            <option value="manual_contact">Manual contact</option>
            <option value="digital_credentials">Digital credentials</option>
            <option value="digital_code">Digital code</option>
          </select>
        </label>
        <label className="span-2">
          Description
          <textarea onChange={(event) => setForm({ ...form, description: event.target.value })} value={form.description} />
        </label>
        <button type="submit">Create product</button>
      </form>
      {message ? <p className="fine-print">{message}</p> : null}
    </section>
  );
}

function PanelTitle({ icon, title }: { icon: React.ReactNode; title: string }) {
  return (
    <div className="panel-title">
      {icon}
      <h2>{title}</h2>
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>;
}

async function api<T>(path: string, options: { method?: string; token?: string; body?: unknown } = {}): Promise<T> {
  const response = await fetch(path, {
    method: options.method || "GET",
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...(options.token ? { Authorization: `Bearer ${options.token}` } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  });

  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const error = await response.json();
      message = error.error || message;
    } catch {
      // Keep the status-based fallback.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

function money(cents: number, currency: string) {
  return new Intl.NumberFormat("zh-CN", { currency: currency || "CNY", style: "currency" }).format(cents / 100);
}

function fulfillmentLabel(strategy: string) {
  return strategy.split("_").join(" ");
}

function submitPaymentForm(formHTML: string) {
  const container = document.createElement("div");
  container.innerHTML = formHTML;
  const form = container.querySelector("form");
  if (!form) return;
  document.body.appendChild(form);
  form.submit();
}
