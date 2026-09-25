# Table-Card Independent

牌桌是一个以终端为完整交互界面的多人桌游项目。大厅、房间、聊天和所有游戏操作均在终端客户端中完成；服务端负责房间状态、规则校验和实时广播。

## 技术栈

- Go 1.26
- Bubble Tea v2、Bubbles v2、Lip Gloss v2：全屏终端应用、输入组件与样式
- `net/http` 和 `coder/websocket`：轻量房间 API 与实时状态传输
- 内存房间状态：本地启动不需要 Docker、Redis 或数据库

## 启动

Windows 双击 `start-table-card.bat`，启动一个终端客户端。启动脚本会构建并后台启动本地服务端；关闭所有客户端后，它会关闭本次启动的服务端。

PowerShell 可指定同时打开的终端客户端数：

```powershell
./start-table-card.ps1 -Clients 3
```

独立运行服务端与客户端：

```powershell
go run ./cmd/table-card-server -listen :1781
go run ./cmd/table-card -server localhost:1781 -name 玩家1
```

多个客户端连到同一服务端后，在一个客户端创建房间，其余客户端输入房间号加入。大厅按 `R` 准备、按 `S` 开始；对局内按 `/` 聊天、按 `Esc` 或 `Del` 返回模式选择。

## 终端操作

- 大厅：方向键选择模式和座位；`E` 编辑名称；`R` 编辑房间号；`C` 创建；`J` 加入；斗地主可按 `B` 开启两个 Sunjiajia 机器人训练。
- 棋盘：方向键移动光标，`Enter` 落子或走棋；国际象棋升变可用 `Q/R/B/N` 选择棋子，围棋用 `P` Pass。
- 卡牌和麻将：按屏幕提示用数字键选择牌，再按操作键出牌或响应。
- `Ctrl+C` 退出客户端。

## 游戏模式

斗地主、骗子酒馆、四川麻将、中国象棋、国际象棋、五子棋、围棋和 UNO 共用终端大厅与实时房间连接。各模式规则实现位于独立的 `internal` 包中，服务端统一通过房间引擎执行操作。

这是持续完善中的游戏实现，部分线下房规与结算选项仍有限制，详见 [`docs/mode-migration-status.md`](docs/mode-migration-status.md)。

## 构建

```powershell
go build ./...
```
