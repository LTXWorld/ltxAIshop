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
