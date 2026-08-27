# Finance API (Node / Express)

A fourth implementation of the Finance Tracker API, beside the .NET one in
`backend/`, the Go one in `backend-go/` and the Python one in `backend-py/`.
All four speak the same HTTP contract, read the same Postgres database, and
accept each other's bearer tokens, so the frontend switches between them at
run time.

Express 5 on Node 24, in TypeScript, with `pg` for Postgres.

## Why it can stand in for the others

- **Passwords** are ASP.NET Core Identity v3 blobs (PBKDF2-HMAC-SHA512,
  100,000 iterations, 16-byte salt, 32-byte subkey). A hash written by any of
  the four verifies in the other three.
- **Tokens** are HS256 with the issuer and audience `finance-tracker`, a 7-day
  lifetime and a one-minute clock leeway. A token minted here is accepted by
  the .NET, Go and Python APIs unchanged.

## The two things JavaScript made hard

**Money.** There is no decimal type, and the usual libraries normalise trailing
zeros away -- `new Decimal('1250.00').toString()` is `'1250'`. The API has to
put `1250.00` on the wire. So `src/domain/money.ts` carries an integer count of
the smallest unit *plus the scale*, and addition keeps the wider of the two
scales, which is what the other backends' decimal libraries do. Rendering goes
through `src/api/rendering.ts`, which writes the JSON document directly rather
than via `JSON.stringify`, because no JavaScript number can distinguish
`1250.00` from `1250`.

**Timestamps.** A `Date` holds milliseconds; the column holds microseconds, and
the other three emit all six digits. `src/domain/instant.ts` keeps the extra
digits alongside a `Date` used for comparison, and the pg driver hands
timestamps over as text so nothing is truncated on the way in.

## Layout

```
src/
  main.ts            process entry: read env, open the pool, serve
  app.ts             express app, CORS, error handling, probes
  core/              config, security (JWT + Identity v3), errors
  domain/            enums, money, instants, calendar arithmetic, validation
  api/               request parsing, JSON rendering, query binding, routers
  services/          business logic -- balances, analytics, recurrence, CSV
  repositories/      SQL, one group per aggregate
```

Dependencies point one way: routers -> services -> repositories. A router does
HTTP only; a repository does queries only.

## Running it

The schema is owned by the dbmate migrations in `db/migrations`, shared with
the other three backends -- nothing here issues DDL.

```bash
npm install
npm run build

DATABASE_URL=postgres://postgres:postgres@localhost:5432/finance \
JWT_SECRET=... \
PORT=8083 \
  node dist/main.js
```

| Variable | Required | Default |
|---|---|---|
| `DATABASE_URL` | yes | — |
| `JWT_SECRET` | no | the shared local-development key |
| `PORT` | no | `8083` |
| `ALLOWED_ORIGINS` | no | `http://localhost:5173` |

`JWT_SECRET` must be the same value the other backends use, or tokens stop
crossing between them.

## Tests

```bash
npm test                      # unit only; the API suite skips
TEST_DATABASE_URL=postgres://... npm test    # everything
```

Unit tests cover the money type, the password blob (including legacy
parameters and malformed input), token rejection paths, `AddMonths` clamping,
balance arithmetic, recurrence and the CSV reader/writer. The API suite drives
the real Express app over supertest against a throwaway Postgres schema built
from `db/migrations`, exactly as the Go and Python suites do.
