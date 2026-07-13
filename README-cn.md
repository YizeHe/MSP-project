# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide-cn.md)

[English](README.md) | **中文** | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

**100% AI-edited** 抗审查、端到端加密的去中心化消息网络原型。

| 层 | 职责 |
|----|------|
| **DTN / UDP P2P mesh** | 加密消息、泛洪、离线携带转发 |
| **公链** | 仅 MST 账本与 burn 凭证（无消息正文） |
| **信令种子** | 节点发现与 **打洞**（不中继聊天） |

## 文档

| 文档 | 语言 |
|------|------|
| **[Guide/guide.md](Guide/guide.md)** | 英文（默认） |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | 中文 |
| [MSP白皮书.md](MSP白皮书.md) | 协议白皮书 |
| `代办.md` … `代办7.md` | 迭代规格（AI 协作痕迹，保留入库） |

## 版本

`1.3.2-daiban7` — 开发请用分支 **`dev`**。

## 一句话架构

```
密封密文 → burner 匿名 burn → BurnTicket → DTN 发送
公链：MST 账本 + burn 哈希 · 信令：仅打洞
```

## 代币经济（代办6）

| 参数 | 值 |
|------|-----|
| 总量（不增发） | **4,226,880** MST |
| 认领池 | **26,880**（210 节点 × **128** MST） |
| 矿工奖励池 | **4,200,000** MST |
| 认领窗口 | 2 年 |

消息 burn 约：direct 2+1 · dtn 10+1 · broadcast 5+1 · alert 20+1 MST。

## 快速开始

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed

$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
.\bin\msp.exe -c claim-genesis   # 可选：本地 DTN 钱包 +128
.\bin\msp.exe -c chain-claim     # 链上 +128（最多 210 名额）
```

完整教程（CLI、双节点、排错）：**[Guide/guide-cn.md](Guide/guide-cn.md)**。

## 测试

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## 许可证

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors。

## 项目立场

本仓库以 **100% AI-edited** 为旗号：规格（`代办*`）、`.learnings/` 与 AI 协作痕迹**不**从仓库抹除。
