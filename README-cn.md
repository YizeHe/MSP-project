# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide-cn.md)

[English](README.md) | **中文** | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

**100% AI-edited** 抗审查消息网络：**DTN/P2P 端到端加密传消息**，**公链只记账 MST**，共识为 **生产向 PoS 核心**（类以太坊 slot/质押选主，**不是**完整 ETH Gasper）。

适合 **封闭内测**；不宣称金融级最终性。

## 架构

```
发消息 → 密封密文 → burner 匿名 burn → BurnTicket → UDP mesh 泛洪
公链   → 余额 / 质押 / burn 哈希 / 出块奖励（无消息正文）
信令   → 仅发现与打洞
```

| 层 | 职责 | 端口 |
|----|------|------|
| **DTN / UDP P2P** | 聊天、泛洪、离线携带 | **UDP**，默认系统随机（`udp_port: 0`） |
| **公链** | 账本 + PoS 出块 | 进程内 + mesh 广播区块/交易 |
| **信令种子** | 打洞，不中继聊天 | **HTTP/HTTPS**（如 443 或自建 8787） |
| **本机 GUI** | 浏览器操作本节点 | **TCP** `127.0.0.1:随机端口` |

## 主网-2（当前）

| | |
|--|--|
| **版本** | `2.0.0-mainnet-pos` |
| **分支** | `dev` |
| **Chain ID** | **`msp-mainnet-2`** |
| **共识** | PoS：10 分钟 slot，质押加权出块人，Ed25519 签块 |
| **免费认领** | **已取消**（无 210×128） |
| **产出** | 仅出块奖励，池 **4,200,000** MST |
| **Bootstrap** | 前 32 个 slot 允许 0 质押激活并出块赚币 |
| **文档** | [MAINNET.md](MAINNET.md) · [genesis/mainnet.json](genesis/mainnet.json) |

> 必须用**新的** `MSP_DATA` 目录；旧 mainnet-1 / 认领数据不兼容。

## 构建

```powershell
git clone https://github.com/YizeHe/MSP-project.git
cd MSP-project
git checkout dev
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
```

## 快速开始

```powershell
$env:MSP_DATA = ".\msp-mainnet-data-v2"

.\bin\msp.exe -c init
.\bin\msp.exe -c seed set http://127.0.0.1:8787   # 可选：手输种子
.\bin\msp.exe -c mine                             # 常驻 mesh + 出块
# 另开终端：
.\bin\msp.exe -c chain-mine
.\bin\msp.exe -c chain
.\bin\msp.exe -c chain-stake 50
.\bin\msp.exe -c chain-genesis
```

GUI：直接 `.\bin\msp.exe`，终端会打印 `http://127.0.0.1:xxxxx/`。

## 手动配置 Seed

```powershell
msp -c seed list
msp -c seed set https://msp.forbiddenx.top
msp -c seed add http://你的公网IP:8787
msp -c seed remove <url>
```

有公网 IP 可自建种子：

```powershell
.\bin\msp-seed.exe -addr 0.0.0.0:8787
# 防火墙放行 TCP 8787；他人：seed set http://你的IP:8787
```

Seed **只打洞**，不转发聊天内容。

## 发消息端口

- **消息：UDP**（`status` 里看 `udp_port`）
- **信令：HTTP(S)**
- **GUI：本机随机 TCP**（`127.0.0.1:0`）

## 常用命令

| 类别 | 命令 |
|------|------|
| 身份 | `init` / `import` / `identity` |
| 种子 | `seed list\|set\|add\|remove` |
| 网络 | `mine` / `start` / `send` / `inbox` / `peers` |
| 链 / PoS | `chain` / `chain-mine` / `chain-stake` / `chain-activate` / `chain-genesis` |
| **已禁用** | 免费 `chain-claim` |

## 测试

```powershell
go test ./... -count=1 -timeout 180s
```

## 文档

- [MAINNET.md](MAINNET.md) — 主网-2 规则  
- [Guide/guide-cn.md](Guide/guide-cn.md) — 中文指南  
- [index.html](index.html) — 落地页  
- [MSP白皮书.md](MSP白皮书.md) — 白皮书  

## 许可证

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors。

## 立场

**100% AI-edited**：`代办*`、`.learnings/` 等 AI 协作痕迹保留入库。
