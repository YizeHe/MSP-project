# MSP / MST Network

**100% AI-edited** 去中心化消息网络原型：公链只记账与 burn 凭证，消息 100% 走 DTN/P2P。

| | |
|--|--|
| **版本** | `1.3.2-daiban7` |
| **分支** | 开发请用 `dev` |
| **使用教程** | **[Guide/guide.md](Guide/guide.md)** ← 从这里开始 |
| **本地二进制** | 构建到 [`bin/`](bin/README.md)（可执行文件 **不** 入库） |

## 一句话架构

```
密封密文 → burner 匿名 burn → BurnTicket → DTN 发送
公链：MST 账本 + burn 哈希 · 信令：仅打洞
```

| 层 | 职责 |
|----|------|
| **公链** | claim / transfer / burn / register / coinbase |
| **DTN** | E2EE 消息、泛洪、离线转发 + BurnTicket |
| **信令** | CF / 本地 seed 打洞（无消息中继） |

## 代币（代办6）

| 参数 | 值 |
|------|-----|
| 总量 | 4,226,880 MST（不增发） |
| 认领 | 128 MST × 210 节点 |
| 矿工池 | 4,200,000 MST |

## 快速构建

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST=1; $env:MSP_CHAIN_FAST=1
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

完整步骤、双节点互发、CLI 手册、故障排除 → **[Guide/guide.md](Guide/guide.md)**。

## 测试

```powershell
$env:MSP_POW_FAST=1; $env:MSP_CHAIN_FAST=1
go test ./... -count=1 -timeout 180s
```

## 文档与 AI 痕迹

本项目明确保留全部 AI 协作产物（**不** 忽略 `代办*.md`、`.learnings/`、`.claude/`）：

- `Guide/guide.md` — 用户指南  
- `MSP白皮书.md` — 协议  
- `代办.md` … `代办7.md` — 迭代规格  
- `.learnings/` — 经验与错误记录  
- `msp-design-dialog.txt` — 设计对话  

## License

TBD
