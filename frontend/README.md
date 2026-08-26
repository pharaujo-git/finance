# Finance Tracker — frontend

React 19 + Vite + TypeScript client for the Finance Tracker API.

## Getting started

```bash
npm install
cp .env.example .env.local   # point the API URLs at your backends
npm run dev                  # http://localhost:5173
```

## API backends

The app can talk to either backend implementation; the choice is made with the
segmented control on the login page and remembered per device under the
`ft.backend` localStorage key (default: `.NET`).

| Variable               | Used by     | Falls back to                            |
| ---------------------- | ----------- | ---------------------------------------- |
| `VITE_API_URL_DOTNET`  | .NET API    | `VITE_API_URL`, then `localhost:5000`    |
| `VITE_API_URL_GO`      | Go API      | `http://localhost:8081`                  |
| `VITE_API_URL`         | .NET only   | `http://localhost:5000`                  |

Each value is an origin **without** a trailing slash and **without** the `/api`
suffix — the fetch wrapper adds it. Settings shows the active backend read-only.

## Scripts

| Script              | What it does                                  |
| ------------------- | --------------------------------------------- |
| `npm run dev`       | Vite dev server                               |
| `npm run build`     | Type-check the project, then build to `dist/` |
| `npm run preview`   | Serve the production build locally            |
| `npm run lint`      | ESLint (flat config, typescript-eslint)       |
| `npm run typecheck` | `tsc -b --noEmit`                             |
| `npm test`          | Vitest (add `-- --run --coverage` for CI)     |

Coverage is written to `coverage/lcov.info` for the SonarQube scan.

## Layout

```
src/
  api/         endpoint functions + TanStack Query hooks
  components/  ui primitives, layout shell, charts, feature forms
  context/     auth, theme and toast providers (state in *-context.ts)
  hooks/       shared editor/confirm/toast/lookup hooks
  lib/         fetch wrapper, formatters, validation helpers
  pages/       one file per route
  test/        Vitest setup, render helpers and API mock
```

## Notes

- **Auth** — the JWT lives in `localStorage`; the fetch wrapper attaches
  `Authorization: Bearer` and, on a `401`, clears it and redirects to `/login`.
- **Cold starts** — the API runs on a free tier, so requests still in flight
  after 2.5s raise a "waking up the server" banner.
- **Dark mode** — class strategy on `<html>`, persisted in `localStorage`,
  seeded from `prefers-color-scheme` on first visit.
- **Money** — always formatted with `Intl.NumberFormat` using the signed-in
  user's currency.
