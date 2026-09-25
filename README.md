# Table-Card Independent

A locally developed multiplayer tabletop game project. This repository starts from a clean Git history and contains a new implementation; it does not import source files, generated code, assets, or Git metadata from another repository.

## Current scope

The first implementation slice defines a standard 54-card deck and an independent landlord-hand classifier/comparison engine. The application and multiplayer layers will be built on top of explicit game-state transitions rather than a renderer-driven game loop.

## Planned structure

- `internal/cards`: card identities, deck creation, shuffling and ordering.
- `internal/landlord`: landlord hand types and round rules.
- `internal/table`: room membership, seats, readiness and game lifecycle.
- `internal/wire`: versioned client/server messages.
- `internal/client` and `internal/server`: transport-facing adapters.
- `internal/ui`: terminal presentation and input handling.
- `internal/games`: independently implemented rules for the remaining modes.

## Build

```powershell
go build ./...
```
