# Table-Card Independent

A locally developed multiplayer tabletop game project. This repository starts from a clean Git history and contains a new implementation; it does not import source files, generated code, assets, or Git metadata from another repository.

## Current scope

The first implementation slice defines a standard 54-card deck, an independent landlord-hand classifier/comparison engine, round transitions, a room lifecycle model, and the first decision-making bot, **Sunjiajia**. The application and multiplayer layers will be built on top of explicit game-state transitions rather than a renderer-driven game loop.

Sunjiajia is a new heuristic player. It evaluates hand structure, ranks, bombs and jokers, avoids taking a teammate's trick when safe, and spends stronger combinations more readily when an opponent is close to going out. Its behavior and tuning are documented in [`docs/sunjiajia-design.md`](docs/sunjiajia-design.md).

## Planned structure

- `internal/cards`: card identities, deck creation, shuffling and ordering.
- `internal/landlord`: landlord hand types and round rules.
- `internal/bot/sunjiajia`: independent landlord bot strategy and move generation.
- `internal/table`: room membership, seats, readiness and game lifecycle.
- `internal/wire`: versioned client/server messages.
- `internal/client` and `internal/server`: transport-facing adapters.
- `internal/ui`: terminal presentation and input handling.
- `internal/games`: independently implemented rules for the remaining modes.

## Build

```powershell
go build ./...
```
