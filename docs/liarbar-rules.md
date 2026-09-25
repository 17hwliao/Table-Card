# Liar's Bar mode

This package implements the four-player hidden-card challenge loop as an isolated game state machine.

- The deck contains six Q, six K, six A, and two Jokers. Each surviving player receives five cards each round.
- A Q, K, or A target is selected for the round. A played card is truthful when it matches the target or is a Joker; one other card makes the whole play a bluff.
- A turn hides the selected 1–3 cards from public snapshots and makes the next living player the only player who can challenge.
- A successful challenge makes the previous player shoot; a failed challenge makes the challenger shoot.
- If a caught bluff includes a Joker, the local house rule fires every living player except the challenger once; the challenge result reports each shot separately.
- Every player receives one fixed random live chamber at game start. Shot count and elimination persist across rounds; cards and target reset after a challenge.
- The game ends when one player remains alive. `Snapshot(viewerID)` only exposes that viewer's own hand.

The rules package is independent from the terminal renderer and room transport. The Bubble Tea client renders each player's private view and sends actions through the shared WebSocket room service.
