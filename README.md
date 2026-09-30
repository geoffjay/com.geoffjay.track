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
- HTTP **basic auth** for the web UI + **bearer API tokens** for `/api/v1`
  (tokens created/revoked on the Settings page; bcrypt-hashed passwords)
- Deployed to **fly.io** with auto-stop (idle machine stops; requests
  cold-start it in ~1s)

## Local development

```sh
npm install          # once: tailwind + daisyui
npm run build       # build assets/styles.css -> internal/web/styles.css
templ generate ./... # regenerate *_templ.go after editing .templ files
go run ./cmd/track   # http://localhost:8080
```

Log in with `geoff` / `geoff-row` or `misty` / `misty-row`
(override via `TRACK_GEOFF_PW` / `TRACK_MISTY_PW`; on Fly use
`fly secrets set TRACK_GEOFF_PW=... TRACK_MISTY_PW=...` then redeploy so
the seeds run — note seeds only apply when a user row is missing).

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `TRACK_DB_PATH` | `data/track.db` | SQLite file |
| `TRACK_ENV` | `development` | `production` → gin release mode |
| `TRACK_REALM` | `rowing miles` | Basic auth realm |
| `TRACK_GEOFF_PW` | `geoff-row` | Seed password for geoff |
| `TRACK_MISTY_PW` | `misty-row` | Seed password for misty |
| `TRACK_PASSWORD_COST` | `10` | bcrypt cost for seeding |

## Tests

```sh
go test ./...
```

- `internal/web/smoke_test.go` boots the real router against a temp SQLite DB
  and exercises auth, dashboard render, check-in create/delete (including
  cross-user delete rejection), history, static, and health routes.
- `internal/api/api_test.go` exercises the JSON API end-to-end: auth,
  metrics/measurements CRUD, the error contract (400 field-scoped
  validation, 403 system rows, 404 not found/cross-user, 409 conflicts),
  and cross-user isolation.
- `internal/web/tokens_test.go` covers the Settings page and bearer
  tokens end-to-end: form create (one-time secret display), bearer
  auth on `/api/v1`, basic auth still working, bearer rejected on the
  UI, garbage-token rejection, expiry validation, and revocation.

## Deploy to Fly

```sh
fly launch --no-deploy   # first time: links app, creates the volume
fly deploy
fly secrets set TRACK_GEOFF_PW=... TRACK_MISTY_PW=...
fly deploy               # restart so new secrets are present at seed time
```

`fly.toml` mounts a 1GB volume at `/data` and runs the smallest machine
(256MB shared-1x) with `auto_stop_machines = "stop"`. With near-zero traffic
the cost is the volume (~$0.15/mo) plus seconds of machine time per visit.

```sql
-- Original rowing tracker (still in use by the web UI).
users(id, username UNIQUE, password_hash, created_at)
api_tokens(id, user_id -> users.id, name, token_hash UNIQUE, expires_at?,
           last_used_at?, created_at)
  -- bearer tokens for /api/v1; hash is SHA-256 of the raw value; the raw
  -- value is shown exactly once at creation; expires_at NULL = never
checkins(id, user_id -> users.id, miles CHECK(miles > 0), rowed_at, created_at)

-- Health & fitness (served by the JSON API).
metrics(id, name UNIQUE, label, unit, category, is_system, created_at)
  -- seeded: weight, height, waist, bicep, quadricep, ... (15 rows)
measurements(id, user_id, metric_id -> metrics.id, value, measured_at, note, created_at)

foods(id, name UNIQUE, brand, serving_size, serving_unit, calories, protein, carbs, fat, is_system, created_at)
  -- seeded: 20 common foods, nutrition per stated serving
meals(id, user_id, name, meal_type, portion, notes, eaten_at, created_at)
meal_items(id, meal_id -> meals.id, food_id -> foods.id?, label, quantity,
           calories, protein, carbs, fat, position, created_at)
  -- food_id NULL = ad-hoc item ("calories only", "calories + macros",
  -- "whole meal, rough portion"); food rows denormalize nutrition at write
  -- time so history survives later food edits.

fasts(id, user_id, started_at, ended_at?, target_hours?, notes, created_at)
  -- ended_at NULL = active fast; one active fast per user.

exercises(id, name UNIQUE, type IN ('weighted','timed'), muscles, equipment, is_system, created_at)
  -- seeded: 24 weighted (back squat, bench press, bicep curl, ...) +
  -- 10 timed (row, jog, bike, ...)
workouts(id, user_id, name, started_at, ended_at?, effort 1-10?, notes, created_at)
workout_entries(id, workout_id -> workouts.id, exercise_id -> exercises.id,
                weight_kg?, reps?, duration_sec?, distance_km?, effort?, notes, position, created_at)
  -- weighted entries carry weight_kg + reps; timed entries carry
  -- duration_sec and optional distance_km.
```

## JSON API (v1)

All endpoints live under `/api/v1`. Authentication is either HTTP basic auth
(the same credentials as the web UI) or an API token:

```
Authorization: Bearer <token>
```

Tokens are created and revoked on the **Settings** page (`/settings`) — give
each one a name and an optional expiration date (empty = never expires). The
raw token is shown exactly once at creation; only its SHA-256 hash is stored.
Revoking a token immediately disables it.

Requests and responses are JSON. List endpoints accept `from`, `to`
(RFC3339 or `YYYY-MM-DD`) and `limit` (default 50, max 200) query parameters
and return `{"data": [...]}`; single-object endpoints return the bare object.

Error envelope: `{"error": "...", "field": "..."}` — `field` appears only on
validation failures (400). 403 = system rows are immutable; 404 = not found or
not owned by the caller (existence is not leaked); 409 = conflict (duplicate
name, second active fast, ending a finished fast).

### Metrics & measurements

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/metrics` | List metric catalog |
| POST | `/api/v1/metrics` | Create custom metric `{name, label, unit, category}` |
| GET | `/api/v1/metrics/:id` | One metric |
| PATCH | `/api/v1/metrics/:id` | Update custom metric |
| DELETE | `/api/v1/metrics/:id` | Delete custom metric (must be unreferenced) |
| GET | `/api/v1/measurements` | List (filter `metric_id`, `from`, `to`, `limit`) |
| POST | `/api/v1/measurements` | Log `{metric_id, value, measured_at?, note?}` |
| GET | `/api/v1/measurements/latest` | Most recent value per metric |
| GET | `/api/v1/measurements/:id` | One measurement |
| PATCH | `/api/v1/measurements/:id` | Update value/measured_at/note |
| DELETE | `/api/v1/measurements/:id` | Delete |

### Foods & meals

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/foods` | List food catalog |
| POST | `/api/v1/foods` | Create custom food `{name, brand?, serving_size, serving_unit, calories, protein, carbs, fat}` |
| GET | `/api/v1/foods/:id` | One food |
| PATCH | `/api/v1/foods/:id` | Update custom food |
| DELETE | `/api/v1/foods/:id` | Delete custom food |
| GET | `/api/v1/meals` | List meals (each with items + totals) |
| POST | `/api/v1/meals` | Create meal `{name?, meal_type?, portion?, notes?, eaten_at?, items: [...]}` |
| GET | `/api/v1/meals/:id` | One meal with items + totals |
| PATCH | `/api/v1/meals/:id` | Update meal; `items` (when present) replaces all items |
| DELETE | `/api/v1/meals/:id` | Delete |
| GET | `/api/v1/meals/:id/items` | Items only |

A meal is entered in whatever granularity is at hand:

- calories only: `{"items": [{"label": "dinner", "calories": 800}]}`
- calories with macros: `{"items": [{"label": "dinner", "calories": 800, "protein": 40, "carbs": 80, "fat": 25}]}`
- individual foods with quantities: `{"items": [{"food_id": 1, "quantity": 200}, {"food_id": 7, "quantity": 1}]}` — nutrition is computed from the food's per-serving values (`food.serving * quantity / serving_size`) and denormalized onto the item
- entire meal, rough portion: `{"portion": "medium", "items": [{"label": "burrito bowl", "calories": 650, ...}]}` — `portion` is `small`/`medium`/`large`

Meal totals (`calories`, `protein`, `carbs`, `fat`) are computed from items.

### Fasting

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/fasts` | List fasts (newest first) |
| POST | `/api/v1/fasts` | Start fast `{started_at?, target_hours?, notes?}` (409 if one is already active) |
| GET | `/api/v1/fasts/active` | The active fast (404 when none) |
| GET | `/api/v1/fasts/:id` | One fast |
| PATCH | `/api/v1/fasts/:id` | Update (absent fields keep current values) |
| POST | `/api/v1/fasts/:id/end` | End fast `{ended_at?}` (defaults to now; 409 if already ended) |
| DELETE | `/api/v1/fasts/:id` | Delete |

Every fast response carries `duration_hours` (ended − started, or started →
now while active).

### Exercises & workouts

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/exercises` | List exercise catalog |
| POST | `/api/v1/exercises` | Create custom exercise `{name, type: "weighted"|"timed", muscles?, equipment?}` |
| GET | `/api/v1/exercises/:id` | One exercise |
| PATCH | `/api/v1/exercises/:id` | Update custom exercise |
| DELETE | `/api/v1/exercises/:id` | Delete (must be unreferenced) |
| GET | `/api/v1/workouts` | List workouts (each with entries) |
| POST | `/api/v1/workouts` | Create workout `{name?, started_at?, ended_at?, effort?, notes?, entries: [...]}` |
| GET | `/api/v1/workouts/:id` | One workout with entries |
| PATCH | `/api/v1/workouts/:id` | Update; `entries` (when present) replaces all |
| POST | `/api/v1/workouts/:id/finish` | Finish workout `{ended_at?}` (409 if already finished) |
| DELETE | `/api/v1/workouts/:id` | Delete |
| GET | `/api/v1/workouts/:id/entries` | Entries only |

Entries are validated against the exercise's type:

- weighted: `{"exercise_id": 1, "weight_kg": 60, "reps": 8}` (weight + reps required)
- timed: `{"exercise_id": 25, "duration_sec": 1800, "distance_km": 5.2}` (duration required, distance optional)

Entry-level `effort` (1–10) and `notes` are optional; workout-level `effort`
(1–10) rates the session overall.

### Example

```sh
curl -u geoff:geoff-row -X POST localhost:8080/api/v1/measurements \
  -H 'Content-Type: application/json' \
  -d '{"metric_id": 1, "value": 82.5}'

curl -u geoff:geoff-row -X POST localhost:8080/api/v1/workouts \
  -H 'Content-Type: application/json' \
  -d '{"name": "push day", "effort": 7, "entries": [
        {"exercise_id": 5, "weight_kg": 60, "reps": 8},
        {"exercise_id": 8, "weight_kg": 12, "reps": 12}]}'
```