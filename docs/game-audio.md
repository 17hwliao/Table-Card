# 十一种终端游戏的专属音乐

每个游戏模式对应一首不同的背景曲；进入模式首页、房间或游戏会切换音轨，返回游戏选择页使用骗子酒馆曲。切换前清理上一首播放器及音效，避免模式间重叠播放。声音默认关闭：一般界面按 M，宝可梦冒险界面按 F9。

全部现用曲目由 **Ragnar Random** 创作，来源页面标注 **CC0 1.0**。音乐不是本项目原创，也不是官方宝可梦、俄罗斯方块等游戏的原声。许可证文本见 `licenses/CC0-1.0.txt`。

| 模式 | 原曲名 | 素材来源 |
| --- | --- | --- |
| 斗地主 | The Town Where I Got the Magic Bottle | Orchestral and World Music Pack |
| 骗子酒馆 | Poison the Sultan with Saltpeter (Slower World Version) | Orchestral and World Music Pack |
| 四川麻将 | The Shaman Is Dancing | Orchestral and World Music Pack |
| 中国象棋 | Samurai Eats Ninja for Breakfast | Orchestral and World Music Pack |
| 国际象棋 | Quest of Magic Cowboy Dude | Orchestral and World Music Pack |
| 五子棋 | It Is Dangerous to Be Lonely Without a Sword | Orchestral and World Music Pack |
| 围棋 | The Tribe Is Mellow | Orchestral and World Music Pack |
| UNO | Adventure Cats Pirate Radio | Fakebit / Chiptune Music Pack |
| 俄罗斯方块 | Reason of the Itch | Fakebit / Chiptune Music Pack |
| 贪吃蛇 | Monkey Dong Goes Bananas | Fakebit / Chiptune Music Pack |
| 宝可梦文字冒险 | Youthful Elf Seeking Adventure (Chiptune) | Fakebit / Chiptune Music Pack |

作者与素材页：[Ragnar Random](https://opengameart.org/users/ragnar-random)、[Orchestral and World Music Pack](https://opengameart.org/content/orchestral-and-world-music-pack)、[Fakebit / Chiptune Music Pack](https://opengameart.org/content/fakebit-chiptune-music-pack)。许可：[CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/)。来源核对日期：2026-10-05。

## 处理与播放

保留每首曲目的完整长度、速度和音高，统一响度至约 -20 LUFS、峰值上限 -2 dBTP，再转为44.1 kHz双声道96 kbps MP3。背景播放器仍然只顺序解码当前一首，不把所有音轨解码到内存；每首结束后从头循环。游戏操作音效沿用程序生成的短音，不需要额外文件。

音乐直接嵌入客户端，房主包和玩家包完整包含，无需网络、在线音乐账号或额外播放器。源下载链接、曲目归属见 `internal/terminal/audio/catalog.json`；原文件与处理后文件的 SHA-256 见 `asset-checksums.json`。这些值用于检查文件一致性，授权以来源页面为准。

## 开发者重新导入

运行 `python -m pip install --target runtime/audio-tools imageio-ffmpeg` 后运行 `python scripts/fetch-mode-music.py`，或指定 `--ffmpeg` 路径。工具缓存原始 OGG 到忽略的 `runtime/music-source`，将处理后的 MP3 和校验清单写入音频目录。导入工具只用于制作资产，玩家运行不依赖 Python 或 FFmpeg。
