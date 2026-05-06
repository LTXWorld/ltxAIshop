import "@testing-library/jest-dom/vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

const productResponse = {
  products: [
    {
      id: 1,
      name: "Product A",
      slug: "product-a",
      description: "Manual contact product",
      priceCents: 9900,
      currency: "CNY",
      status: "published",
      fulfillmentStrategy: "manual_contact",
      imageUrl: ""
    }
  ]
};

describe("App", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const path = input.toString();
        if (path === "/api/products") {
          return jsonResponse(productResponse);
        }
        return jsonResponse({ error: "not found" }, 404);
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the storefront shell with products", async () => {
    render(<App />);

    expect(screen.getByRole("button", { name: /ltxAI Shop/i })).toBeInTheDocument();
    expect(await screen.findByText("Product A")).toBeInTheDocument();
    expect(screen.getByText("Manual contact product")).toBeInTheDocument();
  });

  it("opens the account view", async () => {
    render(<App />);

    await waitFor(() => expect(screen.getByText("Product A")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: "Account" }));

    expect(screen.getAllByRole("button", { name: "Login" }).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Register" })).toBeInTheDocument();
  });
});

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      headers: { "Content-Type": "application/json" },
      status
    })
  );
}
