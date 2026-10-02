# Third-party notices

牌桌 Card Table 本身按本仓库附带的 GNU GPL v3 许可证发布。以下开源组件保留各自许可证；版本见 `go.mod` 和 `go.sum`。

| Component | Use | License / source |
| --- | --- | --- |
| Bubble Tea, Bubbles, Lip Gloss | Terminal UI and styling | MIT · <https://github.com/charmbracelet/bubbletea> |
| coder/websocket | WebSocket transport | ISC · <https://github.com/coder/websocket/blob/master/LICENSE.txt> |
| beep | MP3 playback and audio mixing | BSD-3-Clause · <https://github.com/gopxl/beep> |
| Oto | Audio output backend (transitive dependency) | Apache-2.0 · <https://github.com/ebitengine/oto> |
| PokéAPI factual CSV data | Offline facts for the first 151 species: names, stats, types, capture rates, evolution triggers | BSD-3-Clause · <https://github.com/PokeAPI/pokeapi/blob/master/LICENSE.md> · license retained at `internal/pokemon/POKEAPI_LICENSE.md` (portable packages: `licenses/POKEAPI_LICENSE.md`) |

The MP3 tracks in `internal/terminal/audio` were supplied by the project owner and embedded in the client at the owner's request. Their musical authorship and separate distribution terms are not asserted by the source-code license.

Pokémon and Pokémon character names are trademarks of Nintendo. The text adventure uses Pokémon world/species names as a fan adaptation. Its dialogues, command system, battles and save implementation were written for this project; it does not embed official artwork, audio, ROM data or dialogue scripts. The repository's software license does not claim ownership of third-party characters, trademarks or factual data.

The Gen I capture probability was studied using `pret/pokered`'s documented implementation: <https://github.com/pret/pokered/blob/master/engine/items/item_effects.asm>. No assembly source was copied into the game. This project expresses the aggregate probability in Go and replaces the original visual shake calculation with three independent gates whose combined success probability matches the aggregate.
