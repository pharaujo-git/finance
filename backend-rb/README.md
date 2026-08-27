# Finance API (Ruby / Rails)

A fifth implementation of the Finance Tracker API, beside the .NET one in
`backend/`, the Go one in `backend-go/`, the Python one in `backend-py/` and
the Node one in `backend-node/`. All five speak the same HTTP contract, read
the same Postgres database, and accept each other's bearer tokens, so the
frontend switches between them at run time.

Rails 8.1 in API-only mode on Ruby 4.0, with `pg` for Postgres.

## Why it can stand in for the others

- **Passwords** are ASP.NET Core Identity v3 blobs (PBKDF2-HMAC-SHA512,
  100,000 iterations, 16-byte salt, 32-byte subkey). A hash written by any of
  the five verifies in the other four.
- **Tokens** are HS256 with the issuer and audience `finance-tracker`, a 7-day
  lifetime and a one-minute clock leeway. A token minted here is accepted by
  the .NET, Go, Python and Node APIs unchanged.

## Rails, minus the parts that would own the schema

**No Active Record.** The schema belongs to the backend-neutral dbmate
migrations in `db/migrations`, shared with the other four backends, so this app
must never issue DDL and never own a `schema.rb`. It talks to Postgres through
the `pg` driver directly, behind a small connection pool in
`lib/core/database.rb`, exactly as the Go, Python and Node backends do.

**No Zeitwerk over the domain code.** Rails' autoloader wants one constant per
file under an autoload root. The domain, repository and service code is grouped
by concern rather than by constant, so it lives under `lib/` outside the
autoload paths and is required explicitly, in dependency order, by
`lib/finance.rb`. Controllers stay in `app/controllers` and are autoloaded
normally.

**No credentials.** `config/master.key` is not part of this app; production
derives the `secret_key_base` Rails insists on from `JWT_SECRET`. Nothing here
reads it — there are no cookies, no sessions and no message verifiers.

## The thing Ruby made hard

**Money.** `BigDecimal` is exact, but `BigDecimal("1250.00").to_s` is
`"0.125e4"`, and every route from there back to a string either loses the
trailing zeros or has to be told the scale. The API has to put `1250.00` on the
wire, and `1250` is a different document. So `lib/domain/money.rb` carries an
integer count of the smallest unit *plus the scale*, and addition keeps the
wider of the two scales, which is what the other backends' decimal libraries
do. Rendering goes through `lib/api/rendering.rb`, which writes the JSON
document directly rather than through `JSON.generate`, because no Ruby number
literal survives the round trip.

Timestamps were free by comparison: `Time` keeps nanoseconds, so the six
digits the column holds arrive intact.

## Layout

```
app/controllers/     HTTP only: one controller per aggregate
config/              Rails boot; initializers/container.rb wires the object graph
lib/
  finance.rb         the explicit require order for everything below
  core/              config, database pool, security (JWT + Identity v3)
  domain/            enums, money, instants, calendar arithmetic, validation
  api/               JSON rendering, request parsing, query binding
  services/          business logic -- balances, analytics, recurrence, CSV
  repositories/      SQL, one group per aggregate
```

Dependencies point one way: controllers -> services -> repositories. A
controller does HTTP only; a repository does queries only.

## Running it

```bash
bundle install

DATABASE_URL=postgres://postgres:postgres@localhost:5432/finance \
JWT_SECRET=... \
PORT=8084 \
  bundle exec puma -C config/puma.rb
```

| Variable | Required | Default |
|---|---|---|
| `DATABASE_URL` | yes | — |
| `JWT_SECRET` | no | the shared local-development key |
| `PORT` | no | `8084` |
| `ALLOWED_ORIGINS` | no | `http://localhost:5173` |

`JWT_SECRET` must be the same value the other backends use, or tokens stop
crossing between them.

## Tests

```bash
bundle exec rspec                                    # unit only; the request suite skips
TEST_DATABASE_URL=postgres://... bundle exec rspec   # everything
bundle exec rubocop                                  # rubocop-rails-omakase
```

Unit specs cover the money type, the password blob (including legacy
parameters and malformed input), token rejection paths, `add_months` clamping,
balance arithmetic, recurrence and the CSV reader/writer. The request specs
drive the real Rails stack against a throwaway Postgres schema built from
`db/migrations`, exactly as the Go, Python and Node suites do.
