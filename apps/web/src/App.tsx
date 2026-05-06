import { Server, ShieldCheck, ShoppingBag } from "lucide-react";
import "./styles.css";

export default function App() {
  return (
    <main className="app-shell">
      <section className="hero">
        <div>
          <p className="eyebrow">MVP Foundation</p>
          <h1>ltxAI Shop</h1>
          <p className="lede">AI product marketplace foundation</p>
        </div>
        <div className="status-panel" aria-label="System foundation status">
          <div>
            <ShoppingBag aria-hidden="true" />
            <span>Storefront</span>
          </div>
          <div>
            <ShieldCheck aria-hidden="true" />
            <span>Admin ready</span>
          </div>
          <div>
            <Server aria-hidden="true" />
            <span>Go API</span>
          </div>
        </div>
      </section>
    </main>
  );
}
