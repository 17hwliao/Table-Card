# 牌桌 Card Table

牌桌是以终端为完整交互界面的多人桌游项目。大厅、房间、聊天和游戏操作都在终端中完成；本地服务端负责房间、规则校验和实时同步。

## 游戏模式

- 斗地主：三人对局，支持 Sunjiajia 机器人训练
- 骗子酒馆：四人诈唬、质疑和淘汰，支持机器人补位
- 四川麻将：四人血战到底、换三张、定缺、碰杠胡、计分和房规设置
- 中国象棋、国际象棋、五子棋、围棋：终端棋盘支持鼠标点击与键盘操作，并可与机器人对弈
- UNO：2–4 人，支持叠罚、跳打、7/0、+4 挑战、房规设置和累计积分

游戏模式、音乐和规则相互隔离。按 `M` 开关音乐，按 `F1` 查看当前模式规则。

## Windows 启动

双击 `start-table-card.bat`，或从 PowerShell 执行：

```powershell
./start-table-card.ps1 -Clients 1
```

脚本会检查本机是否已有兼容服务；没有时会在后台启动本地服务，然后打开指定数量的终端客户端。可再次运行脚本为同一服务增加客户端。服务端会常驻，关闭客户端不会自动停止服务。

```powershell
# 同时打开 4 个本地终端客户端
./start-table-card.ps1 -Clients 4

# 启动时交互输入客户端数量
./start-table-card.ps1 -PromptClients

# 停止脚本启动的本地服务
./scripts/stop-local-server.ps1 -Port 1781

# 安装桌面“启动牌桌.bat”和“牌桌本地测试.bat”入口
./scripts/install-desktop-launcher.ps1

# 生成发给其他 Windows 用户的便携包
./scripts/package-windows.ps1
```

便携包已经包含服务端、客户端和内置音乐。接收者不需要安装 Go、Docker、Redis 或数据库，解压后双击 `start-table-card.bat` 即可开始本机游玩。

桌面上的 `启动牌桌.bat` 会询问要打开几个客户端；`牌桌本地测试.bat` 会在独立的 18781 端口直接打开 4 个客户端，方便测试四人桌。测试后可用 `./scripts/stop-local-server.ps1 -Port 18781` 停止后台服务。

## 多人连接

本机启动多个客户端时，它们会自动连接同一服务端。在一个客户端选择模式后创建房间，其他客户端输入房间号加入；按 `R` 准备，按 `S` 开始。快速匹配等待 15 秒后由机器人补足座位。用户断线超过 30 秒后由机器人接管其座位。

朋友要通过网络连接时，房主需在可访问的电脑上运行服务端并开放所用 TCP 端口，再让玩家连接房主的地址：

```powershell
./start-table-card.ps1 -Server 192.168.1.20:1781
```

公网游玩还需要玩家之间能访问房主网络；本项目当前没有云端匹配或中继服务。

## 操作

- 大厅：`E` 编辑昵称，`←/→` 调整人数；选择快速匹配、创建房间、加入房间或人机练习
- 房间：`R` 准备，`S` 开始，`/` 聊天
- 对局：按界面提示操作，`/` 聊天，`F1` 规则，`F2` 结束后再开一局，`M` 音乐，`Del` 离桌
- 棋盘：鼠标点击棋子和目标点；方向键移动光标，`Enter` 选中或落子
- 斗地主：`←/→` 移动选牌，`Space` 切换选择，`Enter` 出牌，`P` 不出
- 聊天：`Enter` 发送，`Esc` 退出聊天并恢复游戏操作
- `Ctrl+C` 退出客户端

主动离开正在进行的对局会将座位交给机器人。离开等待房间会移除玩家；没有真人玩家的房间会关闭。

## 实现栈

- Go、Bubble Tea v2、Bubbles v2、Lip Gloss v2：终端界面和输入
- `net/http`、`coder/websocket`：房间 API 和实时同步
- 游戏各自独立的规则包：服务端执行合法性校验与机器人的行动
- 本地 JSON 文件：玩家身份、战绩和排行榜
- 内嵌 MP3：发行版本无需额外下载音乐文件

本地游玩不需要 Docker、Redis 或数据库。构建源码需要 Go；Windows 便携包的使用者无需安装 Go。

## 构建与检查

```powershell
go build ./...
go test ./...
```

具体规则范围和当前已知限制见 [`docs/mode-migration-status.md`](docs/mode-migration-status.md)。

## 许可证与来源

本项目源代码依 GNU GPL v3 发布，完整文本见 [`LICENSE`](LICENSE)。第三方组件及由项目所有者提供的音乐素材说明见 [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)。
