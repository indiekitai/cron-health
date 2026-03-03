[English](README.md) | [中文](README.zh-CN.md)

# cron-health

简洁的 Cron 任务监控工具。开源版 [healthchecks.io](https://healthchecks.io)。

## 特性

- **单文件部署** - 无依赖，一个可执行文件搞定
- **SQLite 存储** - 数据本地存储在 `~/.cron-health/data.db`
- **HTTP Ping 端点** - 简单的 GET 请求记录任务状态
- **Cron 表达式** - 支持标准 cron 语法定义监控计划
- **耗时追踪** - 自动记录任务运行时长
- **任务统计** - 查看成功率、耗时趋势和运行历史
- **Wrap 命令** - 一行命令包装 crontab，自动上报状态
- **状态徽章** - SVG 徽章，可嵌入 README 或仪表盘
- **Telegram 通知** - 任务失败时通过 Telegram 告警
- **Webhook 通知** - 状态变化时 POST 到任意 HTTP 端点
- **交互式 TUI** - 终端 UI 仪表盘，实时监控
- **彩色输出** - 一目了然哪些任务正常

## 安装

### 从源码

```bash
go install github.com/indiekitai/cron-health@latest
```

### 本地构建

```bash
git clone https://github.com/indiekitai/cron-health.git
cd cron-health
go build -o cron-health .
```

## 快速开始

```bash
# 初始化配置
cron-health init

# 创建每日备份监控（每 24 小时一次，1 小时宽限期）
cron-health create daily-backup --interval 24h --grace 1h

# 或使用 cron 表达式
cron-health create nightly-backup --cron "0 2 * * *" --grace 1h

# 启动 HTTP 服务
cron-health server --port 8080 &

# 在 cron 任务末尾加上 ping
# 0 2 * * * /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup

# 查看状态
cron-health list
cron-health status daily-backup

# 启动交互式仪表盘
cron-health tui
```

## 命令

### `cron-health init`

初始化配置文件 `~/.cron-health/config.yaml`。

### `cron-health create <name>`

创建新的监控项。

```bash
# 每小时一次的监控
cron-health create hourly-task --interval 1h

# 带宽限期（迟到 5 分钟才标记为 DOWN）
cron-health create daily-backup --interval 24h --grace 1h

# 使用 cron 表达式（每天凌晨 2 点）
cron-health create nightly-backup --cron "0 2 * * *" --grace 1h

# 支持的时间格式：30s, 5m, 1h, 1d, 1h30m
```

### `cron-health list`

列出所有监控项及当前状态。

```bash
cron-health list
```

输出：
```
NAME            STATUS  INTERVAL/CRON  LAST PING      NEXT EXPECTED
daily-backup    ● OK    24h            2 hours ago    in 21h55m
nightly-backup  ● OK    0 2 * * *      8 hours ago    in 15h30m
hourly-sync     ● LATE  1h             1 hour ago     overdue
weekly-report   ● DOWN  0 9 * * 1      10 days ago    overdue
```

支持 `--json` 输出。

### `cron-health status [name]`

查看某个监控项的详细状态。支持 `--quiet`（仅输出状态字符串）和 `--json` 模式。

### `cron-health delete <name>`

删除监控项及其历史记录。

### `cron-health logs <name>`

查看 Ping 历史。

```bash
cron-health logs daily-backup --limit 10
```

### `cron-health stats <name>`

查看详细统计，包括运行次数、成功率和耗时指标。

```bash
cron-health stats daily-backup --days 7
```

### `cron-health wrap <name> <command>`

用自动 Ping 包装命令，最简单的监控方式。

```bash
# 旧写法（手动 curl）
0 2 * * * curl -s http://localhost:8080/ping/backup/start && /opt/backup.sh && curl -s http://localhost:8080/ping/backup || curl -s http://localhost:8080/ping/backup/fail

# 新写法（wrap 命令）
0 2 * * * cron-health wrap backup "/opt/backup.sh" --server http://localhost:8080
```

### `cron-health badge <name>`

生成 SVG 状态徽章。

- **绿色** - OK（按计划运行）
- **黄色** - LATE（Ping 逾期）
- **红色** - DOWN（超过宽限期）
- **灰色** - Unknown（监控项不存在）

### `cron-health tui`

启动交互式终端 UI 仪表盘。

快捷键：`j/↓` 下移、`k/↑` 上移、`Enter` 查看详情、`a` 添加、`d` 删除、`r` 刷新、`q/Esc` 退出。

### `cron-health server`

启动 HTTP 服务接收 Ping。

```bash
cron-health server --port 8080
cron-health server --daemon  # 后台运行
```

## HTTP 端点

| 端点 | 描述 |
|------|------|
| `GET /ping/<name>` | 记录成功的 Ping |
| `GET /ping/<name>/fail` | 记录失败的 Ping |
| `GET /ping/<name>/start` | 记录任务开始（可选） |
| `GET /health` | 健康检查 |
| `GET /api/monitors` | 所有监控项（JSON） |
| `GET /badge/<name>.svg` | 状态徽章（SVG） |

## 状态转换

```
[上次 Ping] --> [间隔已过] --> LATE --> [宽限期已过] --> DOWN
     ^                                                    |
     |_________________ [收到 Ping] ______________________|
```

## Exit Code

| 退出码 | 含义 |
|--------|------|
| 0 | 所有监控项正常 |
| 1 | 至少一个 LATE |
| 2 | 至少一个 DOWN |

## 配置

配置文件位于 `~/.cron-health/config.yaml`：

```yaml
server_port: 8080

notify_on:
  - late
  - down
  - recovered

notifications:
  telegram:
    enabled: true
    bot_token: "123456:ABC-DEF..."
    chat_id: "-1001234567890"

  webhook:
    enabled: true
    url: "https://your-webhook-url.com/hook"
```

## 作为系统服务运行

创建 `/etc/systemd/system/cron-health.service`：

```ini
[Unit]
Description=cron-health monitoring server
After=network.target

[Service]
Type=simple
User=your-user
ExecStart=/usr/local/bin/cron-health server --port 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable cron-health
sudo systemctl start cron-health
```

## License

MIT License
