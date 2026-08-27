# Database migrations

The schema lives here, as plain SQL, and is applied with
[dbmate](https://github.com/amacneil/dbmate). No backend owns the schema: the .NET API
only creates tables for the throwaway SQLite database used by local development and the
test suite. Whenever `DATABASE_URL` is set the API issues no DDL and no seeding at all --
every Postgres database (local Docker, Neon prod) gets its shape from the files in
`db/migrations`.

## Install dbmate

```sh
brew install dbmate
```

Or grab the static binary (no Homebrew, useful in CI):

```sh
curl -fsSL -o /usr/local/bin/dbmate \
  https://github.com/amacneil/dbmate/releases/latest/download/dbmate-macos-arm64
chmod +x /usr/local/bin/dbmate
```

Swap `macos-arm64` for `macos-amd64` or `linux-amd64` as needed.

## Local Postgres

The repo ships a `docker-compose.yml` with a `postgres:16` service:

```sh
docker compose up -d
```

It publishes port 5432. If you already run Postgres locally on 5432 the container will be
shadowed -- stop the local server, or publish the container on another port and adjust
`DATABASE_URL` to match.

## Running migrations

dbmate looks for `./db/migrations` relative to the current directory, so **run it from the
repo root**:

```sh
cd <repo root>
dbmate up
```

`DATABASE_URL` is read from the environment; dbmate also loads `db/.env` (see
`db/.env.example`, and note `db/.env` is gitignored):

```sh
cp db/.env.example db/.env
```

Useful commands, all from the repo root:

| Command | What it does |
| --- | --- |
| `dbmate up` | Applies every pending migration |
| `dbmate status` | Lists applied and pending migrations |
| `dbmate down` | Rolls back the most recent migration |
| `dbmate new add_something` | Creates `db/migrations/<timestamp>_add_something.sql` |

### Suppress the schema.sql dump

By default `dbmate up` rewrites `db/schema.sql` after every run, which is pure churn here
because the migrations themselves are the source of truth. Turn it off:

```sh
export DBMATE_NO_DUMP_SCHEMA=true
```

Put that line in `db/.env` so it applies to every local run, or pass `--no-dump-schema` on
the command line. CI should set the same variable.

## Adding a migration

```sh
cd <repo root>
DBMATE_NO_DUMP_SCHEMA=true dbmate new add_transaction_currency
```

That writes a dated, empty file with `-- migrate:up` / `-- migrate:down` sections. Fill in
both -- the down section is what makes `dbmate down` usable. Keep the same conventions as
the baseline: quoted PascalCase identifiers, integer columns for enums (the ordinal from
`FinanceTracker.Domain`), `numeric(18,2)` for money, `timestamp with time zone` for dates.

The two files already here:

- `0001_baseline.sql` -- the schema as EF Core's `EnsureCreated()` produced it, captured
  from `pg_dump --schema-only` and verified to be byte-identical. EF's `PK_*` / `IX_*`
  names are preserved deliberately so an already-deployed database matches it exactly.
- `0002_seed_default_categories.sql` -- the 18 shared default categories the API used to
  upsert on startup. Every insert is guarded by `NOT EXISTS`, so it is safe to replay and
  is a no-op against a database that already has them.

## Production baseline

**Already done — kept for the record.** This section used to describe a one-time fixup for a
Neon database whose seven tables had been created by the old `EnsureCreated()` path: `0001`
had to be marked applied by hand or `dbmate up` would fail on it.

That situation never actually arose. The Neon database was empty when CI first ran
`migrate-prod`, so dbmate applied `0001_baseline.sql` and `0002_seed_default_categories.sql`
normally and production has been a plain dbmate database from the start. Nothing needs marking
by hand.

To check the state of production at any time, against the **unpooled** endpoint:

```sh
DATABASE_URL="$DATABASE_URL_UNPOOLED" dbmate status
```

Both migrations should read `[X]`, with `Pending: 0`.
