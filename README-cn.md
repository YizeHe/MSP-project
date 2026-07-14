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

`2.0.0-mainnet-pos` — 分支 **`dev`** · 链 ID **`msp-mainnet-2`**

| | |
|--|--|
| 免费认领 210×128 | **已取消** |
| 产出 | **仅 PoS 出块奖励**（池 4,200,000 MST） |
| 出块 | **10 分钟** slot，权益加权出块人 |
| 说明 | [MAINNET.md](MAINNET.md) |

## 一句话架构

```
密封密文 → burner 匿名 burn → BurnTicket → DTN 发送
公链：PoS 账本 + burn 哈希 · 信令：仅打洞
```

## 快速开始

```powershell
go build -o bin/msp.exe ./cmd/msp
$env:MSP_DATA = ".\msp-mainnet-data"   # 主网-2 请用新目录
.\bin\msp.exe -c init
.\bin\msp.exe -c mine                  # 常驻出块
.\bin\msp.exe -c chain-mine            # 提议区块
.\bin\msp.exe -c chain-stake 50
```

完整教程：**[Guide/guide-cn.md](Guide/guide-cn.md)** · **[MAINNET.md](MAINNET.md)**

## 测试

```powershell
go test ./... -count=1 -timeout 180s
```

## 许可证

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors。

## 项目立场

本仓库以 **100% AI-edited** 为旗号：规格（`代办*`）、`.learnings/` 与 AI 协作痕迹**不**从仓库抹除。
