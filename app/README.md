# Track — mobile client

Flutter app (Android-first, iOS supported) for the
[com.geoffjay.track](../README.md) JSON API. Authentication is a single API
bearer token — no other method. Create one on the server's Settings page.

## Flow

- **First launch** shows a token-entry gate: server URL + API token. The
  token is verified against `/api/v1/metrics` before being stored; on
  success it persists (shared_preferences) and the main UI loads.
- **Subsequent launches** detect the stored token and go straight to the
  main UI.
- **Sign out** (Settings sheet) clears the stored token and returns to the
  gate.

## Tabs

| Tab | Covers |
|---|---|
| **Measure** | Latest value per metric, full log, per-metric history, custom-metric catalog (system metrics are locked, matching the API) |
| **Meals** | Meal log with computed totals, item-level detail; items reference catalog foods (amount in the food's serving unit — g/ml/egg/slice — nutrition is scaled server-side) or carry ad-hoc calories/macros; food catalog CRUD |
| **Fasts** | Active-fast card with live elapsed timer and target progress, start/end, history with edit/delete |
| **Workouts** | Session log with entry summaries; entries are type-aware (weighted: weight × reps; timed: duration + optional distance); finish an active session; exercise catalog CRUD |

## Layout

```
lib/
  main.dart                  # TrackApp: AppStateScope + gate vs shell
  lib/src/
    app_state.dart           # token/baseUrl persistence, derived ApiClient, AppStateScope
    api_client.dart          # typed /api/v1 client (all CRUD surfaces)
    models.dart              # JSON models mirroring internal/fitness structs
    widgets.dart             # formatting helpers + LoadableList (refresh/retry/empty)
    screens/
      login.dart             # token-entry gate
      home_shell.dart        # bottom-nav shell, per-tab Navigators
      measurements.dart      # + metric catalog & per-metric history
      meals.dart             # + food catalog & item builder
      fasts.dart             # + live active-fast timer
      workouts.dart          # + exercise catalog & entry builder
      settings.dart          # server info + sign-out sheet
```

## Development

```sh
flutter pub get
flutter run                # device or emulator; Android is the priority target
flutter analyze            # clean
flutter test               # widget tests (token gate behavior)
```

For a local server on an Android emulator use `http://10.0.2.2:8080`
(host loopback as seen from the emulator); cleartext HTTP is enabled in the
Android manifest and ATS allows local networking on iOS.

## End-to-end smoke

`tool/e2e_smoke_test.dart` runs the real `ApiClient` against a locally
booted server and exercises every CRUD surface (metrics, measurements,
foods, meals with computed totals, fasts including the active/merge/end
lifecycle, exercises, workouts with type-aware entries, and the 400/404
error contract):

```sh
# server: go run ./cmd/track  (default :8080; pick a free port if needed)
TOKEN=$(scripts/create-test-token.sh)   # or insert a SHA-256 hash manually
TRACK_E2E_URL=http://localhost:8199 \
TRACK_E2E_TOKEN=$TOKEN dart run tool/e2e_smoke_test.dart
```

## Platform notes

- Android application id: `com.geoffjay.track.app` (label "Track").
- `INTERNET` permission and `usesCleartextTraffic` are set in the main
  manifest (release builds need the permission; the debug template alone
  only covers debug builds).
- iOS display name "Track"; `NSAllowsLocalNetworking` permits http dev
  servers without disabling ATS globally.