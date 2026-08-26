# Deployment runbook

Two targets, one pipeline:

- **Frontend** (Vite/React) → Vercel, deployed by `.github/workflows/ci.yml` after the SonarQube gate passes.
- **Backend** (.NET API `finance-api`, Go API `finance-api-go`) → Render, both deployed from
  `render.yaml` (Docker blueprint). Same database, same JWT secret, same endpoints.
- **Database** → Neon Postgres, injected as `DATABASE_URL`.
- **Schema** → `db/migrations`, applied by the `migrate-prod` CI job with dbmate. Neither API
  issues DDL against Postgres.

---

## 1. GitHub secrets

Add these under **Settings → Secrets and variables → Actions → New repository secret**.

| Secret | Where it comes from |
| --- | --- |
| `VERCEL_TOKEN` | <https://vercel.com/account/tokens> → *Create Token*. Scope it to the team that owns the project; copy it once, it is not shown again. |
| `VERCEL_ORG_ID` | Run `vercel link` inside `frontend/`, then read `.vercel/project.json` → `orgId`. |
| `VERCEL_PROJECT_ID` | Same file → `projectId`. |
| `NEON_DATABASE_URL` | Neon → **Connect** → the **direct / unpooled** connection string (see below). |

`NEON_DATABASE_URL` must be the **unpooled** endpoint — the host *without* `-pooler`, exposed
by Neon as `DATABASE_URL_UNPOOLED`. Neon's pooled endpoint is PgBouncer in transaction pooling
mode: it hands out a different backend connection per transaction, does not keep session state,
and does not reliably carry DDL, advisory locks or `SET` statements. dbmate wants exactly those
things — it takes a lock, runs `CREATE TABLE` / `ALTER TABLE`, and records the version in one
session. Point it at the pooler and migrations fail intermittently, which is the worst way for
them to fail. The APIs keep using the pooled string in `DATABASE_URL`; only the migration job
uses the unpooled one.

If the secret is absent the `migrate-prod` job logs a `::notice::` and does nothing, exactly
like `deploy` does without `VERCEL_TOKEN` — the pipeline stays green on a fork or a fresh clone.

```bash
cd frontend
npx vercel login
npx vercel link          # pick/create the project
cat .vercel/project.json # {"orgId":"team_...","projectId":"prj_..."}
```

`.vercel/` is local-only — keep it out of git.

The `deploy` job runs `vercel pull` → `vercel build --prod` → `vercel deploy --prebuilt --prod`,
so the build happens on the runner and Vercel only receives the prebuilt output. Any environment
variables the frontend needs at build time must be set in the **Vercel** project settings for the
*Production* environment — `vercel pull` brings them down before the build:

| Variable | Value |
| --- | --- |
| `VITE_API_URL_DOTNET` | Origin of the `finance-api` Render service. |
| `VITE_API_URL_GO` | Origin of the `finance-api-go` Render service. |
| `VITE_API_URL` | Legacy single-backend variable. Still honoured, but only as the fallback for `VITE_API_URL_DOTNET`; the Go backend ignores it. |

Origins only — scheme and host, **no** trailing slash and **no** `/api` suffix; the fetch wrapper
adds the path. The backend a visitor talks to is chosen in the UI and stored per device, so both
variables need to be set for the switch to work in production.

## 2. Render (backends)

`render.yaml` declares two web services and one environment group:

| Resource | What it is |
| --- | --- |
| `finance-api` | .NET API. `backend/Dockerfile`, listens on `$PORT` (default `8080`). |
| `finance-api-go` | Go API. `backend-go/Dockerfile`, listens on `$PORT` (default `8081`). |
| `finance-shared` | Environment group holding `JWT_SECRET`, attached to both services. |

> **Read this before you apply the blueprint.** `finance-api` currently runs with a `JWT_SECRET`
> that Render generated *per service*. The blueprint moves that variable into the `finance-shared`
> group, and Render will generate a **new** value for the group. Copy the **current** value out of
> `finance-api` → **Environment** first, then paste it into
> **Environment → Environment Groups → finance-shared → `JWT_SECRET`**, replacing the generated
> one. Skip this and the signing key changes once: every issued token stops validating and every
> logged-in user is signed out. Do it once, and the two APIs mint tokens the other accepts.

1. **New → Blueprint**, connect this repository, pick the branch `main`. Render reads `render.yaml`
   and proposes both services (Docker, free plan, Frankfurt) plus the `finance-shared` group.
2. On the confirmation screen fill in the `sync: false` variables **for each service**:
   - `DATABASE_URL` — the Neon **pooled** connection string (see §3). The same value on both.
   - `ALLOWED_ORIGINS` — the production Vercel URL, `https://finance-beta-umber.vercel.app` (the production Vercel domain)
     (comma-separate if you also want preview domains). This one is easy to set on one service
     and forget on the other; the browser then works against one backend and CORS-fails on the
     other, which looks like the Go API being broken.
3. Fix up `JWT_SECRET` in the group as described in the note above.
4. After the first deploy, open **Service → Settings → Build & Deploy** on *each* service and
   switch **Auto-Deploy** to **After CI Checks Pass**. That makes Render wait for this repo's
   GitHub checks — including the SonarQube gate — before shipping a new image, instead of
   deploying on every push.
5. Health checks hit `GET /health` on both; the container must listen on `$PORT` (Render injects
   it). The Go image fails fast and exits non-zero when `DATABASE_URL` is missing, so a
   misconfigured deploy shows up as a boot loop rather than a service answering 500s.

Free-plan caveat: each service spins down after ~15 minutes idle, so the first request after a
quiet spell takes a few seconds. Two services means two cold starts, independently.

## 3. Neon (`DATABASE_URL`)

Either route works; pick one.

- **Vercel Marketplace** — in the Vercel dashboard: **Storage → Create Database → Neon**. Vercel
  provisions the project and injects `DATABASE_URL` (plus `DATABASE_URL_UNPOOLED` and the
  `PG*` variants) into the *frontend's* Vercel environment. Copy the value into Render — Render
  has no access to Vercel's environment.
- **Direct at [neon.tech](https://neon.tech)** — create a project, then **Dashboard → Connect** and
  copy the connection string. Choose the *pooled* endpoint (host contains `-pooler`) for a web API.

Append `?sslmode=require` if your driver does not default to TLS. Use a Neon **branch** for
staging rather than a second project — branches are cheap and copy-on-write.

Rotating the password in Neon means updating `DATABASE_URL` on **both** Render services (and
`NEON_DATABASE_URL` in GitHub, if the unpooled credentials changed too) and redeploying.

### Migrations

The schema is owned by `db/migrations` and applied with dbmate; see **[db/README.md](../db/README.md)**
for the tool, the conventions and the local workflow. Two things matter for production:

- **The one-time baseline.** The existing Neon database was created by the old EF Core
  `EnsureCreated()` path, so `0001_baseline.sql` would fail against it. It has to be marked as
  already applied, by hand, once, before the first deploy of the migration-managed backends —
  the exact commands are in
  [db/README.md → One-time production baseline](../db/README.md#one-time-production-baseline-existing-neon-database).
  Until that is done the `migrate-prod` job will fail.
- **After that it is automatic.** The `migrate-prod` job runs `dbmate up` against
  `NEON_DATABASE_URL` on every `main` push that clears the quality gate, before the frontend
  deploys. Adding a migration is the whole deployment step.

## 4. The SonarQube quality gate

The `sonarqube` job is the gatekeeper: `migrate-prod` has `needs: sonarqube` and `deploy` has
`needs: [sonarqube, migrate-prod]`, so nothing migrates and nothing ships if the gate is red.
It scans three projects — `finance-backend` (.NET), `finance-backend-go` (Go) and
`finance-frontend` — each with its own coverage report and its own gate verdict.

**Ephemeral by design.** Each run boots `sonarqube:community` as a GitHub Actions service
container on `localhost:9000`. There is no persistent server, no data to maintain, and no token to
rotate — but also no history, so *New Code* conditions have nothing to diff against.

That is why the job builds a custom gate (`ci-gate`) out of **overall-code** conditions and makes
it the instance default before any project exists, so all three projects inherit it:

| Metric | Fails when |
| --- | --- |
| `bugs` | `> 0` |
| `vulnerabilities` | `> 0` |
| `security_rating` | worse than A |
| `reliability_rating` | worse than A |
| `coverage` | `< 60%` |
| `duplicated_lines_density` | `> 5%` |

Two details the bootstrap step handles that are easy to miss when reproducing this by hand:

- A newly created gate is **seeded with the built-in "Sonar way" new-code conditions**
  (`new_violations > 0`, `new_coverage < 80`, …). The step deletes them first, so the gate is
  exactly the six conditions above and nothing else.
- Recent builds default to **Multi-Quality Rule mode**, where `bugs`/`vulnerabilities`/`*_rating`
  are treated as legacy metrics. They are still computed and still accepted as gate conditions, so
  no mode switch is needed; the step falls back to the `software_quality_*` equivalents if a future
  build starts rejecting them.

A condition whose metric has no value for a project (e.g. `coverage` when nothing reported
coverage at all) is skipped rather than failed — which is why the coverage floor only bites once
a coverage report is actually being uploaded.

Every scanner runs with `sonar.qualitygate.wait=true`, so `dotnet-sonarscanner end` and the two
`sonar-scanner` invocations block until the server finishes computing and exit non-zero on a red
gate. A final step prints each project's status and per-condition actuals into the job summary.

Coverage inputs:

- backend — `dotnet test --collect:"XPlat Code Coverage;Format=opencover"` →
  `**/coverage.opencover.xml` (`sonar.cs.opencover.reportsPaths`)
- backend-go — `go test -race -covermode=atomic -coverprofile=coverage.out` from the `backend-go`
  job, downloaded as an artifact into `backend-go/coverage.out`
  (`sonar.go.coverage.reportPaths`). The profile names files by import path
  (`github.com/pharaujo/finance/backend-go/...`) while the scanner indexes them relative to the
  repo root, so the step strips the module prefix before scanning — without that the report
  imports as zero coverage.
- frontend — Vitest lcov from the `frontend` job, downloaded as an artifact into
  `frontend/coverage/lcov.info` (`sonar.javascript.lcov.reportPaths`)

The .NET scan excludes `frontend/**` and `backend-go/**` so the other two languages are analysed
once, by the scanner that also has their coverage.

### Reproducing a scan locally

```bash
# 1. Server (keep it running; first boot takes ~1 min)
docker run --rm -d --name sonarqube \
  -p 9000:9000 -e SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true \
  sonarqube:community
until curl -s localhost:9000/api/system/status | grep -q '"status":"UP"'; do sleep 5; done

# 2. Log in at http://localhost:9000 (admin/admin), change the password when
#    prompted, then My Account -> Security -> generate a token.
export SONAR_TOKEN=squ_xxx

# 3. Backend
dotnet tool install --global dotnet-sonarscanner --allow-roll-forward
dotnet-sonarscanner begin /k:"finance-backend" \
  /d:sonar.host.url=http://localhost:9000 /d:sonar.token=$SONAR_TOKEN \
  /d:sonar.cs.opencover.reportsPaths="**/coverage.opencover.xml"
dotnet build backend/FinanceTracker.sln -c Release
dotnet test backend/FinanceTracker.sln -c Release --no-build \
  --collect:"XPlat Code Coverage;Format=opencover"
dotnet-sonarscanner end /d:sonar.token=$SONAR_TOKEN

# 4. Backend (Go), from the repo root
(cd backend-go && go test -race -covermode=atomic -coverprofile=coverage.out ./...)
# Strip the module prefix so the scanner can match the indexed files.
# (BSD/macOS sed; GNU sed, as in CI, takes plain `-i`.)
sed -i '' 's#^github\.com/pharaujo/finance/##' backend-go/coverage.out
npx --yes @sonar/scan@5 \
  -Dsonar.projectKey=finance-backend-go \
  -Dsonar.host.url=http://localhost:9000 -Dsonar.token=$SONAR_TOKEN \
  -Dsonar.sources=backend-go -Dsonar.tests=backend-go \
  -Dsonar.test.inclusions="**/*_test.go" \
  -Dsonar.exclusions="**/*_test.go" \
  -Dsonar.go.coverage.reportPaths=backend-go/coverage.out

# 5. Frontend (from the repo root, after `npm test -- --run --coverage` in frontend/)
npx --yes @sonar/scan@5 \
  -Dsonar.projectKey=finance-frontend \
  -Dsonar.host.url=http://localhost:9000 -Dsonar.token=$SONAR_TOKEN \
  -Dsonar.sources=frontend/src -Dsonar.tests=frontend/src \
  -Dsonar.test.inclusions="frontend/src/**/*.test.*,frontend/src/**/*.spec.*" \
  -Dsonar.exclusions="frontend/src/**/*.test.*" \
  -Dsonar.javascript.lcov.reportPaths=frontend/coverage/lcov.info

# 6. Tear down
docker stop sonarqube
```

To mirror CI exactly, recreate `ci-gate` locally (Quality Gates → Create → **delete the inherited
new-code conditions** → add the six overall-code conditions above → *Set as Default*), or lift the
bootstrap step's script straight out of the workflow and point `SONAR_URL` at your container.

Both scanners auto-provision their own JRE from the SonarQube server, so a local Java install is
not required.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `sonarqube` job times out waiting for UP | Elasticsearch failed to start; check the service container logs in the job output. |
| Gate fails only on `coverage` | New code landed without tests — the threshold is 60% overall, per project. |
| Vercel deploy: "Project not found" | `VERCEL_ORG_ID` / `VERCEL_PROJECT_ID` do not match the token's team. |
| Render 502 after deploy | The app is not binding to `$PORT`, or `/health` is not reachable. |
| CORS errors in the browser | `ALLOWED_ORIGINS` on Render does not include the exact Vercel origin (scheme + host, no trailing slash) — check *both* services, they are configured separately. |
| CORS errors only on the Go backend | `ALLOWED_ORIGINS` was set on `finance-api` but not on `finance-api-go`. |
| Everyone logged out after a deploy | `JWT_SECRET` changed. The `finance-shared` group value was not seeded from the old per-service secret. |
| Token from one backend rejected by the other | The two services are not reading the same `finance-shared` group. |
| `finance-api-go` restarts in a loop | `DATABASE_URL` is unset or unreachable; the Go API refuses to start rather than serving errors. Check the deploy logs for `config: DATABASE_URL is required`. |
| `migrate-prod` fails on `0001_baseline.sql` | The production database was never baselined — see [db/README.md](../db/README.md#one-time-production-baseline-existing-neon-database). |
| `migrate-prod` fails intermittently on DDL | `NEON_DATABASE_URL` points at the pooled (`-pooler`) host. Use the unpooled one. |
