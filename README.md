# Finance Tracker

Personal finance app: accounts, transactions, budgets, goals, dashboards, and reports.

The API exists twice — once in .NET, once in Go — against the same database, the same
password hashes and the same JWT secret. The frontend picks which one to talk to.

## Stack

| Layer    | Tech                                                        | Hosting            |
| -------- | ----------------------------------------------------------- | ------------------ |
| Frontend | React 19, Vite, TypeScript, Tailwind, TanStack Query        | Vercel             |
| Backend  | ASP.NET Core 10 Web API, EF Core, JWT auth                  | Render (Docker)    |
| Backend  | Go 1.26, Gin, pgx, JWT auth (same endpoints, same tokens)   | Render (Docker)    |
| Database | PostgreSQL (Neon) in prod, SQLite for .NET local dev/tests  | Neon               |
| Schema   | Plain SQL migrations applied with dbmate                    | `db/migrations`    |
| Quality  | SonarQube Community (ephemeral, in CI) + quality gate       | GitHub Actions     |

## Repository layout

```
frontend/            React app (Vercel project root)
backend/             .NET API: FinanceTracker.{Domain,Application,Infrastructure,Api,Tests}
backend-go/          Go API: cmd/api + internal/{domain,application,http,infrastructure}
backend-py/          Python API: FastAPI, app/{domain,services,repositories,api}
db/                  SQL migrations (dbmate) — the schema's source of truth
docs/                Deployment runbook
.github/             CI pipeline: build → test → migrations → SonarQube gate → deploy
render.yaml          Render blueprint for all three APIs
docker-compose.yml   Local Postgres on :5432
```

## Local development

```bash
# 1. Postgres + schema (from the repo root)
docker compose up -d
dbmate up                    # see db/README.md for install and DATABASE_URL

# 2. .NET API (http://localhost:5000; SQLite unless DATABASE_URL is set)
cd backend && dotnet run --project FinanceTracker.Api

# 3. Go API (http://localhost:8081; DATABASE_URL is required)
cd backend-go && DATABASE_URL=postgres://postgres:postgres@localhost:5432/finance go run ./cmd/api

# 4. Python API (http://localhost:8082; DATABASE_URL is required)
cd backend-py && uv venv .venv && uv pip install --python .venv/bin/python -e ".[dev]"
DATABASE_URL=postgres://postgres:postgres@localhost:5432/finance \
  .venv/bin/uvicorn app.main:build --factory --port 8082

# 5. Frontend (http://localhost:5173)
cd frontend && npm install && npm run dev
```

All three APIs can run at once. The login page has a segmented control that picks the
backend, remembered per tab; the origins come from `VITE_API_URL_DOTNET`,
`VITE_API_URL_GO` and `VITE_API_URL_PYTHON` (see `frontend/.env.example`). They accept
each other's tokens, so a session survives switching. No API creates or seeds Postgres
tables — whenever `DATABASE_URL` is set the schema comes from `db/migrations` and nothing
else.

## Pipeline

Every push/PR runs frontend lint/tests/build, .NET build/tests, Go gofmt/vet/golangci-lint,
Python ruff/mypy/pytest
and race tests, and applies `db/migrations` to a throwaway Postgres (checking that a second
`dbmate up` is a no-op and that every migration rolls back). It then boots an ephemeral
SonarQube Community server inside the workflow, scans all three projects with coverage, and
fails the run if the quality gate fails. On `main`, and only after the gate passes,
production migrations run against Neon and the frontend deploys to Vercel; the two Render
services redeploy themselves from the blueprint.
