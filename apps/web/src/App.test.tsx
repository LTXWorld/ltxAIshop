import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "./App";

describe("App", () => {
  it("renders the storefront shell", () => {
    render(<App />);

    expect(screen.getByRole("heading", { name: "ltxAI Shop" })).toBeInTheDocument();
    expect(screen.getByText("AI product marketplace foundation")).toBeInTheDocument();
  });
});
