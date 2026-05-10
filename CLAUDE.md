# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, run, test

```
go build ./cmd/caliban           # main daemon
go build ./cmd/tempestHist2Db    # backfill last 5 days of 1-min obs
go build ./cmd/migratesql2sql    # one-off historical migration; not part of normal flow
go run ./cmd/caliban             # run without installing

go vet ./...
gofmt -w .

go test ./tempest/                                    # all tempest tests
go test ./tempest/ -run TestRawToObsIndexMapping      # offline; no API needed
go test ./tempest/ -run TestGetStation                # one live-API test
```

`TestRawToObsIndexMapping` is the only offline test. Everything else in `tempest/` hits the live Tempest REST/WS endpoints. `TestSubscribeObservations` waits for three real WebSocket messages and can block several minutes during calm weather.

## Configuration

Viper, file name `caliban`, type `yaml`. Loaded from different paths depending on binary:

- `cmd/*/main.go` reads `$HOME/.config/caliban.yml`.
- `tempest/tempest_test.go` reads `caliban.yml` from the `tempest/` package directory.

Required keys: `tempest-token`, `tempest-deviceId`, `windy-apiKey`. Optional: `windy-stationId` and `windy-stationPassword` (both required iff `windy-v2: true`), `windy-v2` (default `false`), `db-path` (default `wx.DefaultPath()` — see below). Note `tempest-stationId` is no longer read by the daemon (kept in older configs for reference). The `.gitignore` excludes `*.yml`.

## Architecture

Data flow for the main daemon (`cmd/caliban/main.go`):

```
Tempest WS  →  tempest.SubscribeObservations(ctx, …)  →  <-chan Observation
                                                              │
                              wx.Store.Save(deviceId, obs)  ←─┤   (every obs)
                                                              │
                              windy.Sender.Send(obs)         ←┘   (≥5 min apart)
```

Things that aren't obvious from a single file:

- **`tempest.SubscribeObservations` is context-driven.** It accepts a `ctx`, sets a 90-second read deadline that's bumped by both incoming messages and pong responses, sends client pings every 30s, and closes its channel on either context cancel or read failure. The daemon's outer `for` loop reconnects with exponential full-jitter backoff (1s → 30s) — this is what stops a single network blip from killing the process. There are no `panic`s in the daemon's hot path.
- **The 5-minute Windy throttle lives in `windy.Sender.Send` (windy/windy.go).** It's keyed off observation timestamps (not wall clock). Returns `windy.ErrThrottled` (a sentinel `var`, match with `errors.Is`) when too soon. The daemon doesn't track `lastTimestamp` itself anymore.
- **`windy.Sender` supports two endpoints.** Default is the legacy POST/JSON `pws/update/{key}` URL (Windy is retiring this at end of 2026). `sender.EnableV2(stationID, password)` switches to GET `api/v2/observation/update` with a Bearer header. **Auth is different per endpoint:** legacy uses the account API key in the URL path; v2 uses the per-station password (auto-generated, visible on the My Stations page in Windy — explicitly NOT the account API key, which v2 rejects with "Provided password is invalid"). **Precip semantics differ too:** v2 wants mm-since-local-midnight, legacy wanted mm-in-last-15-min. The daemon picks `obs.LocalDayRainAccumulation` vs `obs.RainAccumulation` based on the `windy-v2` config flag. **Pressure** is sent in Pascals on both paths; the daemon converts from Tempest's millibars at observation-build time.
- **The sqlite store is `wx.Store`.** Construct with `wx.Open(path)` and `defer Close()`. `wx.DefaultPath()` resolves to `$XDG_DATA_HOME/caliban/tempest.db` or `$HOME/.local/share/caliban/tempest.db` if XDG is unset (which it usually is on macOS / default Linux).
- **No column-order coupling at the SQL layer.** `wx/helpers.go` declares `columns []string` once, and INSERT/SELECT statements are built from it; struct field order is independent of column order. The Tempest *wire* format is still positional — `RawToObs` decodes by index — but `tempest_test.go:TestRawToObsIndexMapping` locks that mapping in, so reordering `Observation` fields fails the test instead of silently corrupting history.
- **Duplicate inserts are silenced via `INSERT OR IGNORE`**, not by string-matching the UNIQUE-constraint error.
- **`cmd/migratesql2sql/`** is a frozen historical artifact (it merged early per-device DBs into the unified one). Don't change it; don't run it unless intentionally migrating again.

No CI, no linter config beyond `go vet`.
