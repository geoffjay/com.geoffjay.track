# Tracking App

A personal API and mobile app for tracking health and fitness, not intended to
be used by anyone other than me.

## Stack

- **Go + gin + [templ](https://templ.guide)** — single binary serving the UI
- **SQLite** (`modernc.org/sqlite`, pure Go — no CGO) persisted to a Fly volume
- **daisyUI 5** components via [templ-ui](https://github.com/geoffjay/templ-ui)
  (AppShell container + component library)
- **Charts** via [templ-charts](https://github.com/geoffjay/templ-charts)
  (server-side SVG line + bar)
- HTTP **basic auth** (bcrypt-hashed, two seeded users)
- Deployed to **fly.io** with auto-stop (idle machine stops; requests
  cold-start it in ~1s)

## Local development

```sh
npm install          # once: tailwind + daisyui
npm run build       # build assets/styles.css -> internal/web/styles.css
templ generate ./... # regenerate *_templ.go after editing .templ files
go run ./cmd/track   # http://localhost:8080
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `TRACK_DB_PATH` | `data/track.db` | SQLite file |
| `TRACK_ENV` | `development` | `production` → gin release mode |
| `TRACK_REALM` | `rowing miles` | Basic auth realm |
| `TRACK_PASSWORD_COST` | `10` | bcrypt cost for seeding |

## Tests

```sh
go test ./...
```

`internal/web/smoke_test.go` boots the real router against a temp SQLite DB
and exercises auth, dashboard render, check-in create/delete (including
cross-user delete rejection), history, static, and health routes.

## Deploy to Fly

```sh
fly launch --no-deploy   # first time: links app, creates the volume
fly deploy
```

`fly.toml` mounts a 1GB volume at `/data` and runs the smallest machine
(256MB shared-1x) with `auto_stop_machines = "stop"`. With near-zero traffic
the cost is the volume (~$0.15/mo) plus seconds of machine time per visit.

## Data model

```sql
users(id, username UNIQUE, password_hash, created_at)
checkins(id, user_id -> users.id, miles CHECK(miles > 0), rowed_at, created_at)
```

Miles are cumulative per user; the dashboard shows totals, a 30-day
cumulative line chart, and the last 7 days grouped bars.
