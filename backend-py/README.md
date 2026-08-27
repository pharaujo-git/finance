# Finance API (Python / FastAPI)

A third implementation of the Finance Tracker API, alongside the .NET one in
`backend/` and the Go one in `backend-go/`. All three speak the same HTTP
contract, read the same Postgres database, and accept each other's bearer
tokens, so the frontend can switch between them at run time.

## Why it can stand in for the others

Two things had to match byte for byte, and both are covered by tests:

- **Passwords** are ASP.NET Core Identity v3 blobs (PBKDF2-HMAC-SHA512,
  100,000 iterations, 16-byte salt, 32-byte subkey). A hash written by any of
  the three verifies in the other two, and a blob using weaker parameters
  verifies *and* reports that it should be rewritten.
- **Tokens** are HS256 with the issuer and audience `finance-tracker`, a
  7-day lifetime and a one-minute clock leeway. A token minted here is
  accepted by the .NET and Go APIs unchanged.

The response bodies match too, down to the JSON scale of money (`1250.00`
stays `1250.00`), the camelCase enum names, the RFC 9457 problem documents and
the PascalCase keys of a validation 400.

## Layout

```
app/
  main.py            FastAPI app factory, CORS, health, service document
  core/              config, security (JWT + Identity v3), errors, DI
  domain/            enums, money, calendar arithmetic, validation rules
  api/               request parsing, JSON rendering, query binding, routers
  services/          business logic -- balances, analytics, recurrence, CSV
  repositories/      SQL, one class per aggregate
```

Dependencies point one way: routers → services → repositories. A router does
HTTP only; a repository does queries only.

## Running it

The API needs Postgres. The schema is owned by the dbmate migrations in
`db/migrations`, shared with the other two backends -- nothing here issues DDL.

```bash
uv venv .venv && uv pip install --python .venv/bin/python -e ".[dev]"

DATABASE_URL=postgres://postgres:postgres@localhost:5432/finance \
JWT_SECRET=... \
PORT=8082 \
  .venv/bin/uvicorn app.main:build --factory --port 8082
```

| Variable | Required | Default |
|---|---|---|
| `DATABASE_URL` | yes | — |
| `JWT_SECRET` | no | the shared local-development key |
| `PORT` | no | `8082` |
| `ALLOWED_ORIGINS` | no | `http://localhost:5173` |

`JWT_SECRET` must be the same value the other backends use, or tokens stop
crossing between them.

## Tests

```bash
.venv/bin/python -m pytest
```

The suite is pure: it covers the password blob (including legacy parameters
and malformed input), token issuing and rejection, money rounding vectors,
`AddMonths` clamping, enum round-trips, the validation wording, balance
arithmetic, recurrence materialisation and the CSV reader/writer. None of it
needs a database.

## A note on parity

The API was diffed endpoint by endpoint against the running Go and .NET
backends. One difference is worth recording: for an account with a zero
balance, **the Go API emits `"balance":0` while .NET and this API emit
`"balance":0.00`**. The scale comes from the `numeric(18,2)` column, so .NET's
form is the faithful one and the Go backend is the outlier. It is harmless to
the frontend, which parses both as a number.
