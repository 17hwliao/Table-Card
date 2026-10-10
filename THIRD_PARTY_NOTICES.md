# Third-party notices

牌桌 Card Table 本身按本仓库附带的 GNU GPL v3 许可证发布。以下开源组件保留各自许可证；版本见 `go.mod` 和 `go.sum`。

| Component | Use | License / source |
| --- | --- | --- |
| Bubble Tea, Bubbles, Lip Gloss | Terminal UI and styling | MIT · <https://github.com/charmbracelet/bubbletea> |
| coder/websocket | WebSocket transport | ISC · <https://github.com/coder/websocket/blob/master/LICENSE.txt> |
| beep | MP3 playback and audio mixing | BSD-3-Clause · <https://github.com/gopxl/beep> |
| Oto | Audio output backend (transitive dependency) | Apache-2.0 · <https://github.com/ebitengine/oto> |
| PokéAPI factual CSV data | Offline facts for the first 151 species: names, stats, types, capture rates, evolution triggers | BSD-3-Clause · <https://github.com/PokeAPI/pokeapi/blob/master/LICENSE.md> · license retained at `internal/pokemon/POKEAPI_LICENSE.md` (portable packages: `licenses/POKEAPI_LICENSE.md`) |

The eleven MP3 recordings currently embedded in `internal/terminal/audio` are adaptations of music by Ragnar Random, published under CC0 1.0 at OpenGameArt.org. Source titles, original download URLs, mode assignments, processing details and checksums are retained in `docs/game-audio.md`, `internal/terminal/audio/catalog.json` and `internal/terminal/audio/asset-checksums.json`. The music's license is separate from the repository's GPL source-code license. CC0 text is retained at `licenses/CC0-1.0.txt` and included in both portable packages. Older owner-supplied recordings are superseded in the current distribution.

Music sources: <https://opengameart.org/content/orchestral-and-world-music-pack> and <https://opengameart.org/content/fakebit-chiptune-music-pack>. Composer: Ragnar Random. The imported versions retain full compositions, are normalized in loudness and encoded as 44.1 kHz stereo MP3 at 96 kbps; pitch and tempo are unchanged.

Pokémon and Pokémon character names are trademarks of Nintendo. The text adventure uses Pokémon world/species names as a fan adaptation. Its dialogues, command system, battles and save implementation were written for this project; it does not embed official artwork, audio, ROM binaries or dialogue scripts. The repository's software license does not claim ownership of third-party characters, trademarks or factual data.

The Gen I capture probability was studied using `pret/pokered`'s documented implementation: <https://github.com/pret/pokered/blob/master/engine/items/item_effects.asm>. No assembly source was copied into the game. This project expresses the aggregate probability in Go and replaces the original visual shake calculation with three independent gates whose combined success probability matches the aggregate.

The October 2026 Pokémon data correction also uses factual tables from `pret/pokered`: first-generation base stats and growth groups, level-up learnsets, move powers/accuracy/PP and Red-version wild encounter slots. Sources and exact map bindings are recorded in `scripts/rebuild-pokemon-data.py` and `docs/pokemon-text-adventure.md`. The generator reads these tables into compact JSON; the Go battle and networking implementations are written in this repository. No ROM binary, assembly engine, official graphics, music or dialogue script is distributed. Chinese species and move names come from PokéAPI factual CSV tables under the retained BSD-3-Clause notice above.
