# CloudSecGuard — Web Platform

Full documentation: see [README.md](../README.md) in the root of the repository.

## Quick Start (Docker)

```bash
cd web
cp .env.example .env    # optional: add Google OAuth credentials
docker compose up -d --build
```

- Web UI: http://localhost:3000
- Backend API: http://localhost:8080

Check backend logs for the initial admin credentials generated on first start:

```bash
docker compose logs backend | grep -i "admin\|created\|password"
```

## Dev Mode

**Backend:**
```bash
DATABASE_URL="postgres://<user>:<password>@localhost:5432/infra_audit?sslmode=disable" \
ASSETS_DIR="$(pwd)/../assets" \
DATA_DIR="$(pwd)/data" \
go run ./backend/
```

**Frontend:**
```bash
cd frontend
npm install
npm run dev   # http://localhost:5173
```

The Vite dev server proxies `/api` to `localhost:8080`.

## Stack

| Layer | Technology |
|-------|-----------|
| Frontend | React 18, TypeScript, Vite, TailwindCSS, shadcn/ui |
| Backend | Go 1.22, chi router, pgx/v5 |
| Database | PostgreSQL 16 |
| Auth | JWT (access + refresh tokens), optional Google OAuth, TOTP MFA |
| Deploy | Docker Compose (3 containers: nginx, backend, postgres) |
