# Table-Card Independent

A locally developed multiplayer tabletop game project. This repository starts from a clean Git history and contains a new implementation; it does not import source files, generated code, assets, or Git metadata from another repository.

## Current scope

The first implementation slice defines a standard 54-card deck, an independent landlord-hand classifier/comparison engine, the first decision-making bot, **Sunjiajia**, a Liar's Bar rules engine, and a small browser lobby backed by an HTTP API. The application is built on explicit game-state transitions rather than a renderer-driven game loop.

Sunjiajia is a new heuristic player. It evaluates hand structure, ranks, bombs and jokers, avoids taking a teammate's trick when safe, and spends stronger combinations more readily when an opponent is close to going out. Its behavior and tuning are documented in [`docs/sunjiajia-design.md`](docs/sunjiajia-design.md).

## Planned structure

- `internal/cards`: card identities, deck creation, shuffling and ordering.
- `internal/landlord`: landlord hand types and round rules.
- `internal/bot/sunjiajia`: independent landlord bot strategy and move generation.
- `internal/liarbar`: hidden-card challenges, fixed-chamber roulette and elimination state.
- `internal/games`: shared mode registry and engine interface used to attach independent rule packages to rooms.
- `internal/table`: room membership, seats, readiness and game lifecycle.
- `internal/wire`: versioned client/server messages.
- `internal/client` and `internal/server`: transport-facing adapters.
- `internal/ui`: terminal presentation and input handling.
- `internal/games`: independently implemented rules for the remaining modes.

## Build

```powershell
go build ./...
```

## Run locally

Install Go, then run `start-table-card.bat` on Windows or `./start-table-card.ps1` in PowerShell. Open <http://localhost:8080>. Docker is not required. To choose another port, run `go run ./cmd/table-card -listen :8090`.

The lobby currently supports mode selection and room creation, joining, readiness, and start checks. The game API exposes private views and action dispatch for the landlord and Liar's Bar engines. Their interactive table screens are not built yet; the other modes remain listed as planned and cannot start a game session yet.

For an active room, `GET /api/rooms/{code}/state?playerId=...` returns that player's view. `POST /api/rooms/{code}/action` accepts `{ "playerId": "...", "action": { ... } }`; each engine owns its own action schema and validates turn order and private information.
