# Mode migration status

This repository implements each mode inside its own rules package and attaches it through `internal/games`. A Bubble Tea terminal client sends actions over the room WebSocket; the Go server validates them and sends each player a private view.

## Available in the lobby

| Mode | Current playable slice | Remaining rules work |
| --- | --- | --- |
| Landlord | Three-player game, hand classification, bidding/play and Sunjiajia training | More bot tuning and full match/settlement polish |
| Liar's Bar | Four seats, hidden cards, bluff/challenge, shots and elimination | More animation and round presentation |
| Gomoku | 15×15 keyboard cursor, move placement and win/draw checks | Optional opening rules and rematch flow |
| Chinese chess | Keyboard cursor and server-side legal move/check rules | Repetition and draw-claim rules |
| International chess | Keyboard cursor, legal movement, check, castling, en passant, promotion choice, mate/stalemate, repetition and fifty-move draw | Chess clock and claim-based draw controls |
| Go | 19×19 keyboard cursor, captures, suicide prevention, simple ko, pass and Chinese-style area count with 6.5 komi | Dead-stone negotiation, configurable board/scoring rules and match review |
| UNO | 108-card deck, color/number/actions, draw-to-playable, stacking, 7/0, +4 challenge, UNO catch, last-card penalties and 500-point rounds | Opening wild-card choice/rules, wrong-call option and configurable house rules |
| Sichuan Mahjong | 108 suited tiles, exchange-three and missing-suit selection, missing-suit discard priority, discard claims, multi-Hu, peng, concealed/direct/added gang, rob-gang, self-draw, blood-battle continuation, dealer rotation, and basic flow settlement | Exchange-three is mandatory; configurable cap, complete fan/roots calculation, and accurate maximum-fan ready settlement remain |

## Known implementation choices

- The terminal lobby lists modes registered by the server. Mode state is validated by the server; client-side key handling is not treated as authorization.
- A game view only includes the requesting player's private hand. Public seat counts and discards are shared.
- UNO starts with a color or number card as its top discard; opening wild cards are returned to the draw pile. This is the simplified opening rule for now.
- Mahjong currently uses a fixed four-human seat table. There is no automated Mahjong opponent or room rule editor yet. Its settlement is a playable subset and should not be treated as a fully verified implementation of every rule in the supplied Chengdu house rules.
- The current table uses a simple in-memory room lifecycle. Closing a terminal client disconnects it but does not remove the player from an active room.
- User-provided background tracks are kept under `internal/terminal/audio`. Terminal music playback and the per-mode M-key toggle still need to be connected.

## Terminal presentation

- The `cmd/table-card` executable is the interactive terminal client. The `cmd/table-card-server` executable hosts the room API and WebSocket service.
- Game-specific controllers live under `internal/terminal/modes`; they render snapshots and translate keyboard input to server actions. The game engines remain the authority for legal moves.
- The Windows launcher builds the server and client, starts a local server if one is not already responding, and opens the requested number of terminal clients. Docker and Redis are not required.

## Next implementation order

1. Polish terminal input guidance and board/card layouts at narrow console sizes.
2. Complete Mahjong's detailed fan, root, and maximum-fan ready settlement, then expose house-rule options.
3. Add server-side forfeit/reconnect and rematch flows for active rooms.
4. Add automated opponents to modes that need solo practice.
