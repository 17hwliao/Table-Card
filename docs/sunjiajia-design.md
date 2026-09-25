# Sunjiajia bot design

Sunjiajia is the computer-player design for the independent Table-Card codebase. The implementation is original Go code and uses the local game models; it does not load or embed code from the earlier project.

## Decisions

### Bidding

`Bid` assigns strength to high single cards, jokers, twos, and repeated ranks. It maps the resulting score to a 0–3 bid. Keeping this function deterministic makes bidding easy to tune from observed games.

### Move generation

`Choose` builds legal candidate shapes from ranks in the hand: singles, pairs, triples, bombs, the joker rocket, triple attachments, four-card attachments, straights, consecutive pairs, airplanes, and airplane wings. Every generated selection is passed through the same `landlord.Classify` function used by the round engine, so candidate generation cannot define a second, inconsistent rule set.

### Move evaluation

For each candidate, the bot estimates the cost of the remaining hand. It favors removing low singles and preserving connected groups, bombs, and the joker pair. When responding to a trick, it adds a cost for using a high rank or a bomb. It will normally pass instead of spending a bomb when a cheaper legal response exists.

When the opposing team is leading, farmers usually pass to preserve their teammate's trick. They will contest it when the teammate is close to going out. This is a small positional heuristic, not a search-based perfect-play solver.

## Inputs and limitations

The caller supplies the bot's seat, landlord seat, each player's remaining card count, and the current trick. The bot returns a selected set of cards or `nil` to pass. It does not own a network connection, room state, timer, or UI.

The initial version is deterministic and single-ply. It does not yet account for hidden-card probabilities, score/risk settings, farmer signaling, or long-horizon search. Those can be added behind this interface without coupling the strategy to the UI or transport.

### Liar's Bar

The same agent also has a simple Liar's Bar policy. It chooses target cards and Jokers when available, otherwise bluffs with a small number of cards. Its challenge threshold rises with the number of hidden cards and with an opponent's shot count, and becomes more conservative when the bot itself is close to six shots. The hidden chamber is never part of the bot's input, so the policy cannot cheat by reading the random outcome.
