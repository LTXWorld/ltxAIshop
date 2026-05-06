# ltxAI Shop Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the runnable foundation for the ltxAI Shop MVP: Go API, React/Vite frontend, PostgreSQL connectivity, Docker Compose, Nginx reverse proxy, and baseline CI.

**Architecture:** The repository is a small monorepo with `apps/api` for the Go backend and `apps/web` for the React/Vite frontend. Docker Compose runs `postgres`, `api`, `web`, and `nginx` locally, while GitHub Actions runs backend and frontend checks before later deployment plans add VPS publishing.

**Tech Stack:** Go 1.22, chi, pgxpool, PostgreSQL 16, golang-migrate style SQL migrations, React 18, TypeScript, Vite, Vitest, Testing Library, Docker Compose, Nginx, GitHub Actions.

---

## Scope

This plan implements only the project foundation from the approved design. It does not implement customer auth, products, cart, Alipay, inventory allocation, delivery records, admin features, or GitHub Actions deployment to the VPS. Those belong in follow-up plans:

- Authentication and authorization plan
- Catalog, cart, and order plan
- Alipay payment and fulfillment plan
- Admin UI plan
- VPS deployment plan

## File Structure

Create this structure:

```text
apps/
  api/
    cmd/api/main.go
    internal/config/config.go
    internal/config/config_test.go
    internal/database/database.go
    internal/health/handler.go
    internal/health/handler_test.go
    internal/httpserver/router.go
    internal/httpserver/router_test.go
    migrations/000001_create_schema_migrations_table.up.sql
    migrations/000001_create_schema_migrations_table.down.sql
    go.mod
    go.sum
    Dockerfile
  web/
    src/App.tsx
    src/App.test.tsx
    src/main.tsx
    src/styles.css
    index.html
    package.json
    package-lock.json
    tsconfig.json
    tsconfig.node.json
    vite.config.ts
    Dockerfile
deploy/
  nginx/default.conf
.github/
  workflows/ci.yml
.env.example
docker-compose.yml
README.md
```

Responsibilities:

- `apps/api/cmd/api/main.go`: process entrypoint, config loading, database pool, HTTP server startup.
- `apps/api/internal/config`: environment configuration with validation and defaults.
- `apps/api/internal/database`: PostgreSQL pool creation and ping helper.
- `apps/api/internal/health`: `/api/healthz` handler.
- `apps/api/internal/httpserver`: router wiring and middleware.
- `apps/api/migrations`: SQL migrations source directory.
- `apps/web`: React/Vite frontend shell and test setup.
- `deploy/nginx/default.conf`: local and VPS-ready reverse proxy shape.
- `.github/workflows/ci.yml`: backend and frontend validation.
- `docker-compose.yml`: local orchestration for Postgres, API, web dev server, and Nginx.

---

### Task 1: Initialize Backend Module And Config Package

**Files:**

- Create: `apps/api/go.mod`
- Create: `apps/api/internal/config/config.go`
- Create: `apps/api/internal/config/config_test.go`

- [ ] **Step 1: Create the Go module**

Create `apps/api/go.mod`:

```go
module github.com/ltxai/shop/apps/api

go 1.22

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/jackc/pgx/v5 v5.5.5
)
```

- [ ] **Step 2: Write the failing config tests**

Create `apps/api/internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ltxai:ltxai@localhost:5432/ltxai_shop?sslmode=disable")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("WEB_ORIGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL should be set")
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL = %q, want http://localhost:8080", cfg.PublicBaseURL)
	}
	if cfg.WebOrigin != "http://localhost:5173" {
		t.Fatalf("WebOrigin = %q, want http://localhost:5173", cfg.WebOrigin)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want missing DATABASE_URL error")
	}
}
```

- [ ] **Step 3: Run config tests to verify they fail**

Run:

```bash
cd apps/api
go test ./internal/config -run TestLoad -count=1
```

Expected: FAIL because `Load` and `Config` are undefined.

- [ ] **Step 4: Implement config loading**

Create `apps/api/internal/config/config.go`:

```go
package config

import (
	"errors"
	"os"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	PublicBaseURL string
	WebOrigin     string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		PublicBaseURL: getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		WebOrigin:     getEnv("WEB_ORIGIN", "http://localhost:5173"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
```

- [ ] **Step 5: Run config tests to verify they pass**

Run:

```bash
cd apps/api
go test ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/go.mod apps/api/internal/config/config.go apps/api/internal/config/config_test.go
git commit -m "feat: add api config foundation"
```

---

### Task 2: Add Health Router

**Files:**

- Create: `apps/api/internal/health/handler.go`
- Create: `apps/api/internal/health/handler_test.go`
- Create: `apps/api/internal/httpserver/router.go`
- Create: `apps/api/internal/httpserver/router_test.go`

- [ ] **Step 1: Write the failing health handler test**

Create `apps/api/internal/health/handler_test.go`:

```go
package health

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want health JSON", body)
	}
}
```

- [ ] **Step 2: Run health test to verify it fails**

Run:

```bash
cd apps/api
go test ./internal/health -count=1
```

Expected: FAIL because `Handler` is undefined.

- [ ] **Step 3: Implement health handler**

Create `apps/api/internal/health/handler.go`:

```go
package health

import (
	"net/http"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}
```

- [ ] **Step 4: Run health test to verify it passes**

Run:

```bash
cd apps/api
go test ./internal/health -count=1
```

Expected: PASS.

- [ ] **Step 5: Write the failing router test**

Create `apps/api/internal/httpserver/router_test.go`:

```go
package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterServesHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want health JSON", body)
	}
}

func TestRouterReturnsNotFoundForUnknownAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
```

- [ ] **Step 6: Run router test to verify it fails**

Run:

```bash
cd apps/api
go test ./internal/httpserver -count=1
```

Expected: FAIL because `NewRouter` is undefined.

- [ ] **Step 7: Implement router wiring**

Create `apps/api/internal/httpserver/router.go`:

```go
package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ltxai/shop/apps/api/internal/health"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/healthz", health.Handler().ServeHTTP)
	return r
}
```

- [ ] **Step 8: Run router tests to verify they pass**

Run:

```bash
cd apps/api
go test ./internal/httpserver -count=1
```

Expected: PASS.

- [ ] **Step 9: Run all backend tests**

Run:

```bash
cd apps/api
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/health apps/api/internal/httpserver
git commit -m "feat: add api health router"
```

---

### Task 3: Add Database Pool And Migration Directory

**Files:**

- Create: `apps/api/internal/database/database.go`
- Create: `apps/api/migrations/000001_create_schema_migrations_table.up.sql`
- Create: `apps/api/migrations/000001_create_schema_migrations_table.down.sql`

- [ ] **Step 1: Write the failing database package compile check**

Create `apps/api/internal/database/database.go` with this temporary interface-only compile target:

```go
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return nil, nil
}

func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return nil
}
```

Run:

```bash
cd apps/api
go test ./internal/database -count=1
```

Expected: PASS because the package compiles, but `Open` is not yet implemented. This step establishes package shape before implementation.

- [ ] **Step 2: Replace database package with real pool logic**

Replace `apps/api/internal/database/database.go` with:

```go
package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := Ping(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database pool is required")
	}
	return pool.Ping(ctx)
}
```

- [ ] **Step 3: Add initial migrations table migration**

Create `apps/api/migrations/000001_create_schema_migrations_table.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE
);
```

Create `apps/api/migrations/000001_create_schema_migrations_table.down.sql`:

```sql
DROP TABLE IF EXISTS schema_migrations;
```

- [ ] **Step 4: Run backend tests**

Run:

```bash
cd apps/api
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/database apps/api/migrations
git commit -m "feat: add database pool foundation"
```

---

### Task 4: Add API Entrypoint And Dockerfile

**Files:**

- Create: `apps/api/cmd/api/main.go`
- Create: `apps/api/Dockerfile`

- [ ] **Step 1: Write the API entrypoint**

Create `apps/api/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ltxai/shop/apps/api/internal/config"
	"github.com/ltxai/shop/apps/api/internal/database"
	"github.com/ltxai/shop/apps/api/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Run backend tests and build**

Run:

```bash
cd apps/api
go test ./... -count=1
go build ./cmd/api
```

Expected: both commands PASS.

- [ ] **Step 3: Add API Dockerfile**

Create `apps/api/Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.19

RUN adduser -D -H -u 10001 appuser
WORKDIR /app

COPY --from=build /out/api /app/api
COPY migrations /app/migrations

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/api"]
```

- [ ] **Step 4: Build the API image**

Run:

```bash
docker build -t ltxai-shop-api:foundation apps/api
```

Expected: image builds successfully.

- [ ] **Step 5: Commit**

```bash
git add apps/api/cmd/api/main.go apps/api/Dockerfile apps/api/go.sum
git commit -m "feat: add api entrypoint"
```

---

### Task 5: Initialize React/Vite Frontend

**Files:**

- Create: `apps/web/package.json`
- Create: `apps/web/package-lock.json`
- Create: `apps/web/index.html`
- Create: `apps/web/src/App.tsx`
- Create: `apps/web/src/App.test.tsx`
- Create: `apps/web/src/main.tsx`
- Create: `apps/web/src/styles.css`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/tsconfig.node.json`
- Create: `apps/web/vite.config.ts`

- [ ] **Step 1: Create frontend package manifest**

Create `apps/web/package.json`:

```json
{
  "name": "@ltxai/shop-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "tsc -b && vite build",
    "lint": "tsc -b --noEmit",
    "test": "vitest run --environment jsdom"
  },
  "dependencies": {
    "@vitejs/plugin-react": "latest",
    "vite": "latest",
    "typescript": "latest",
    "react": "latest",
    "react-dom": "latest",
    "lucide-react": "latest"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "latest",
    "@testing-library/react": "latest",
    "@testing-library/user-event": "latest",
    "@types/react": "latest",
    "@types/react-dom": "latest",
    "vitest": "latest",
    "jsdom": "latest"
  }
}
```

- [ ] **Step 2: Install frontend dependencies**

Run:

```bash
cd apps/web
npm install
```

Expected: `package-lock.json` is created and install completes successfully.

- [ ] **Step 3: Add TypeScript and Vite config**

Create `apps/web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["DOM", "DOM.Iterable", "ES2020"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

Create `apps/web/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "Node",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

Create `apps/web/vite.config.ts`:

```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080"
    }
  },
  test: {
    globals: true,
    setupFiles: []
  }
});
```

- [ ] **Step 4: Add the failing app test**

Create `apps/web/src/App.test.tsx`:

```tsx
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
```

- [ ] **Step 5: Run frontend test to verify it fails**

Run:

```bash
cd apps/web
npm test
```

Expected: FAIL because `App` is undefined or matcher setup is missing.

- [ ] **Step 6: Add React app shell**

Create `apps/web/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ltxAI Shop</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Create `apps/web/src/App.tsx`:

```tsx
import { ShieldCheck, ShoppingBag, Server } from "lucide-react";
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
```

Create `apps/web/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

Create `apps/web/src/styles.css`:

```css
:root {
  color: #172026;
  background: #f6f7f4;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI",
    sans-serif;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
}

.app-shell {
  min-height: 100vh;
  padding: 40px;
}

.hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 360px);
  gap: 32px;
  align-items: center;
  max-width: 1120px;
  margin: 0 auto;
  min-height: calc(100vh - 80px);
}

.eyebrow {
  margin: 0 0 12px;
  color: #496157;
  font-size: 14px;
  font-weight: 700;
  text-transform: uppercase;
}

h1 {
  margin: 0;
  font-size: 56px;
  line-height: 1;
}

.lede {
  margin: 18px 0 0;
  max-width: 560px;
  color: #53615d;
  font-size: 20px;
}

.status-panel {
  display: grid;
  gap: 12px;
}

.status-panel div {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 14px 16px;
  border: 1px solid #d7ddd7;
  border-radius: 8px;
  background: #ffffff;
  color: #27332f;
  font-weight: 700;
}

.status-panel svg {
  width: 22px;
  height: 22px;
  color: #0f766e;
}

@media (max-width: 760px) {
  .app-shell {
    padding: 24px;
  }

  .hero {
    grid-template-columns: 1fr;
    align-content: center;
    min-height: calc(100vh - 48px);
  }

  h1 {
    font-size: 42px;
  }
}
```

- [ ] **Step 7: Add jest-dom setup**

Modify `apps/web/src/App.test.tsx` to include the matcher import at the top:

```tsx
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
```

- [ ] **Step 8: Run frontend checks**

Run:

```bash
cd apps/web
npm test
npm run build
```

Expected: both commands PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/web
git commit -m "feat: add web foundation"
```

---

### Task 6: Add Frontend Dockerfile

**Files:**

- Create: `apps/web/Dockerfile`

- [ ] **Step 1: Add frontend Dockerfile**

Create `apps/web/Dockerfile`:

```dockerfile
FROM node:22-alpine AS deps

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build

WORKDIR /src
COPY --from=deps /src/node_modules ./node_modules
COPY . .
RUN npm run build

FROM nginx:1.25-alpine

COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
```

- [ ] **Step 2: Build the web image**

Run:

```bash
docker build -t ltxai-shop-web:foundation apps/web
```

Expected: image builds successfully.

- [ ] **Step 3: Commit**

```bash
git add apps/web/Dockerfile
git commit -m "feat: add web container"
```

---

### Task 7: Add Compose, Nginx, Environment Example, And README

**Files:**

- Create: `docker-compose.yml`
- Create: `deploy/nginx/default.conf`
- Create: `.env.example`
- Create: `README.md`

- [ ] **Step 1: Add environment example**

Create `.env.example`:

```dotenv
APP_ENV=development
HTTP_ADDR=:8080
DATABASE_URL=postgres://ltxai:ltxai@postgres:5432/ltxai_shop?sslmode=disable
PUBLIC_BASE_URL=http://localhost:8080
WEB_ORIGIN=http://localhost:5173

POSTGRES_DB=ltxai_shop
POSTGRES_USER=ltxai
POSTGRES_PASSWORD=ltxai
```

- [ ] **Step 2: Add Docker Compose**

Create `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-ltxai_shop}
      POSTGRES_USER: ${POSTGRES_USER:-ltxai}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-ltxai}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-ltxai} -d ${POSTGRES_DB:-ltxai_shop}"]
      interval: 5s
      timeout: 5s
      retries: 10

  api:
    build:
      context: ./apps/api
    environment:
      APP_ENV: ${APP_ENV:-development}
      HTTP_ADDR: ":8080"
      DATABASE_URL: ${DATABASE_URL:-postgres://ltxai:ltxai@postgres:5432/ltxai_shop?sslmode=disable}
      PUBLIC_BASE_URL: ${PUBLIC_BASE_URL:-http://localhost:8080}
      WEB_ORIGIN: ${WEB_ORIGIN:-http://localhost:5173}
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "8080:8080"

  web:
    build:
      context: ./apps/web
    ports:
      - "5173:80"

  nginx:
    image: nginx:1.25-alpine
    volumes:
      - ./deploy/nginx/default.conf:/etc/nginx/conf.d/default.conf:ro
    depends_on:
      - api
      - web
    ports:
      - "8088:80"

volumes:
  postgres_data:
```

- [ ] **Step 3: Add Nginx config**

Create `deploy/nginx/default.conf`:

```nginx
server {
    listen 80;
    server_name localhost;

    location /api/ {
        proxy_pass http://api:8080/api/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        proxy_pass http://web:80/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

- [ ] **Step 4: Add README**

Create `README.md`:

```markdown
# ltxAI Shop

AI product sales website foundation for `ltxAIshop.bfsmlt.com`.

## Local Development

Copy environment defaults:

```bash
cp .env.example .env
```

Run the stack:

```bash
docker compose up --build
```

Open:

- Frontend through Nginx: http://localhost:8088
- API health: http://localhost:8088/api/healthz
- Direct API: http://localhost:8080/api/healthz

## Backend Checks

```bash
cd apps/api
go test ./...
go build ./cmd/api
```

## Frontend Checks

```bash
cd apps/web
npm test
npm run build
```
```

- [ ] **Step 5: Run Docker Compose stack**

Run:

```bash
cp .env.example .env
docker compose up --build
```

Expected: Postgres becomes healthy, API listens on `:8080`, web container serves the frontend, and Nginx listens on `:8088`.

- [ ] **Step 6: Verify health through Nginx**

In a second terminal, run:

```bash
curl -i http://localhost:8088/api/healthz
```

Expected: HTTP 200 and body `{"status":"ok"}`.

- [ ] **Step 7: Commit**

```bash
git add .env.example docker-compose.yml deploy/nginx/default.conf README.md
git commit -m "feat: add local compose stack"
```

---

### Task 8: Add GitHub Actions CI

**Files:**

- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Add CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches:
      - master
      - main
  pull_request:

jobs:
  api:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/api
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache-dependency-path: apps/api/go.sum

      - name: Test
        run: go test ./... -count=1

      - name: Build
        run: go build ./cmd/api

  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: apps/web
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: apps/web/package-lock.json

      - name: Install
        run: npm ci

      - name: Test
        run: npm test

      - name: Build
        run: npm run build
```

- [ ] **Step 2: Run local checks that mirror CI**

Run:

```bash
cd apps/api
go test ./... -count=1
go build ./cmd/api
cd ../web
npm ci
npm test
npm run build
```

Expected: all commands PASS.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add foundation checks"
```

---

### Task 9: Final Foundation Verification

**Files:**

- Verify: repository root

- [ ] **Step 1: Run backend verification**

Run:

```bash
cd apps/api
go test ./... -count=1
go build ./cmd/api
```

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run:

```bash
cd apps/web
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 3: Run container verification**

Run from repository root:

```bash
docker compose up --build -d
curl -fsS http://localhost:8088/api/healthz
docker compose down
```

Expected: `curl` prints `{"status":"ok"}` and Docker Compose shuts down cleanly.

- [ ] **Step 4: Inspect git status**

Run:

```bash
git status --short
```

Expected: no uncommitted changes.

---

## Plan Self-Review

- Spec coverage: this plan covers the approved design's scaffold, Go API, React/Vite frontend, PostgreSQL connectivity, Docker Compose, Nginx shape, and CI baseline. Business features remain intentionally split into follow-up plans listed in Scope.
- Placeholder scan: no task relies on an unspecified future implementation.
- Type consistency: Go package names, module path, exported functions, environment variable names, and frontend scripts are consistent across tasks.
