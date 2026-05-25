# InfraJump Web UI

Modern web interface for the InfraJump Infrastructure Audit platform.

## Quick start (Docker)

```bash
cd /path/to/infra-audit/web
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

## Default credentials

| Field    | Value                  |
|----------|------------------------|
| Email    | admin@infrajump.com    |
| Password | InfraJump2026!         |

## Dev mode

**Backend**
```bash
cd /path/to/infra-audit
go mod tidy
DATABASE_URL="postgres://audit:audit123@localhost:5432/infra_audit?sslmode=disable" \
ASSETS_DIR="$(pwd)/assets" \
DATA_DIR="$(pwd)/web/data" \
go run ./web/backend/
```

**Frontend**
```bash
cd web/frontend
npm install
npm run dev      # http://localhost:5173
```

The Vite dev server proxies `/api` to `http://localhost:8080`.

**Database (local)**
```bash
docker run -d \
  --name infrajump-pg \
  -e POSTGRES_DB=infra_audit \
  -e POSTGRES_USER=audit \
  -e POSTGRES_PASSWORD=audit123 \
  -p 5432:5432 \
  postgres:16-alpine
```

## Feature overview

| Feature | Description |
|---------|-------------|
| Connections | Store multiple DigitalOcean accounts with token, project filter, scope mode |
| Test connection | Validates DO token and lists available projects |
| Audit jobs | Runs a full infra audit in the background |
| Live progress | WebSocket stream shows real-time progress |
| Report downloads | Download HTML and DOCX reports when job completes |
| Auditor profile | Customize org name, contact info embedded in every report |
| Custom assets | Upload logo, watermark, footer background per account |
| Theme toggle | Dark / light mode |

## Custom assets

Upload per-user branding via **Settings → Assets**. Falls back to `infra-audit/assets/` defaults when no custom asset is uploaded.
