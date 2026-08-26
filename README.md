# Finance Tracker

Personal finance app: accounts, transactions, budgets, goals, dashboards, and reports.

## Stack

| Layer    | Tech                                                        | Hosting            |
| -------- | ----------------------------------------------------------- | ------------------ |
| Frontend | React 19, Vite, TypeScript, Tailwind, TanStack Query        | Vercel             |
| Backend  | ASP.NET Core 10 Web API, EF Core, JWT auth                  | Render (Docker)    |
| Database | PostgreSQL (Neon) in prod, SQLite for local dev/tests       | Neon               |
| Quality  | SonarQube Community (ephemeral, in CI) + quality gate       | GitHub Actions     |

## Repository layout

```
frontend/   React app (Vercel project root)
backend/    FinanceTracker.Api + FinanceTracker.Tests
.github/    CI pipeline: build → test → SonarQube gate → deploy
render.yaml Render blueprint for the API
```

## Local development

```bash
# API (http://localhost:5000, SQLite by default)
cd backend && dotnet run --project FinanceTracker.Api

# Frontend (http://localhost:5173)
cd frontend && npm install && npm run dev
```

## Pipeline

Every push/PR runs frontend lint/tests/build and backend build/tests, then boots an
ephemeral SonarQube Community server inside the workflow, scans both projects with
coverage, and fails the run if the quality gate fails. Production deploys (Vercel
frontend, Render API) only happen after the gate passes on `main`.
