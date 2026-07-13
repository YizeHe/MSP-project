# MSP / MST Network 使用指南

> **版本**：1.3.2-daiban7  
> **口号**：本仓库代码与文档为 **100% AI-edited**（含 `代办*.md`、`.learnings/`、`.claude/` 等 AI 协作痕迹，均保留入库）。  
> **仓库**：[YizeHe/MSP-project](https://github.com/YizeHe/MSP-project)

本文是面向使用者与二次开发者的**全套引导**：从安装构建到双节点通信、公链认领与挖矿。

---

## 1. 项目是什么

**MSP（Message Security Protocol / MST Network）** 是一套抗审查、端到端加密的去中心化消息网络原型：

| 层 | 职责 | 是否传消息正文 |
|----|------|----------------|
| **DTN / UDP P2P mesh** | 端到端加密消息、泛洪、离线携带转发 | ✅ 仅内存/本地队列，阅后即焚 |
| **公链（economy-only）** | MST 账本、burn 凭证、coinbase 矿工激励 | ❌ 链上只有哈希与金额 |
| **信令种子（Cloudflare Worker / 本地 msp-seed）** | 节点发现 + **打洞** | ❌ 不中继聊天（`/v1/packet` → 410） |

设计原则（见 `MSP白皮书.md` 与 `代办4.md`–`代办7.md`）：

- 消息永不永久留存；链上零可解密内容  
- MST 仅约束发送（burn）；无增发（矿工池预分配方案 B）  
- 打洞成功后可脱离信令继续通信  

---

## 2. 架构一览

```
发送方
  1. AES-GCM 密封明文（一次）
  2. 一次性 burner 地址：主账户 → burner 转账 → burner 提交 burn{MsgType, RefHash}
  3. 本地矿工签发 BurnTicket（预确认）
  4. DTN/P2P 发送 Packet + BurnTicket
接收方
  1. 校验 ticket（签名 / RefHash / 金额 / TTL / burn 在 mempool 或已上链）
  2. 解密 → 收件箱 → 阅后即焚
```

### 2.1 代币经济（代办6）

| 参数 | 值 |
|------|-----|
| 全网总量（不增发） | **4,226,880** MST |
| 认领池 | **26,880**（**210** 节点 × **128** MST） |
| 矿工奖励池 | **4,200,000** |
| 认领窗口 | 2 年 |
| 区块奖励 | 从矿工池释放，约 50 MST/块起，约每 210240 块减半 |

### 2.2 消息 burn 费用（链上）

| 类型 | 说明 | 约需 MST |
|------|------|----------|
| `direct` | 已打洞直传 | 2 + 手续费 1 |
| `dtn` | 泛洪 / 中继 | 10 + 1 |
| `broadcast` | 广播 | 5 + 1 |
| `alert` | 全网告警（需 Alert 密钥） | 20 + 1 |

发送还会走**一次性 burner**（主账户先转账 fund + transfer fee，再 burn）。

### 2.3 本地 DTN 钱包（代办7）

`msp -c mst` / `claim-genesis` 操作的是**本地** `mst-ledger.json`（与链上账户并行）：

- `grant = 128`，`genesis_supply = 4_226_880`  
- **`msp -c start` 不再自动认领**；需手动 `claim-genesis` 或走链上 `chain-claim` / 挖矿  

---

## 3. 环境要求

- **Go** 1.21+（推荐与 `go.mod` 一致）  
- Windows / Linux / macOS  
- 可选：Cloudflare 账号（部署公共信令）；本机可跑 `msp-seed`  
- 可选：SOCKS5（Tor onion 信令）  

---

## 4. 获取与构建

```bash
git clone https://github.com/YizeHe/MSP-project.git
cd MSP-project
# 建议开发分支
git checkout dev
```

### 4.1 编译到 `bin/`

**Windows (PowerShell)：**

```powershell
$env:CGO_ENABLED = "0"
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
go build -o bin/msp-p2p-test.exe ./cmd/msp-p2p-test
```

**Linux / macOS：**

```bash
go build -o bin/msp ./cmd/msp
go build -o bin/msp-seed ./cmd/msp-seed
go build -o bin/msp-p2p-test ./cmd/msp-p2p-test
```

说明见 [`bin/README.md`](../bin/README.md)。**可执行文件不进 Git**，请本地构建。

### 4.2 实验室加速环境变量

```powershell
$env:MSP_POW_FAST = "1"      # 降低 PoW 难度，实验室用
$env:MSP_CHAIN_FAST = "1"    # 缩短出块间隔（约 15s）
$env:MSP_REQUIRE_TICKET = "0" # 关闭收包强制 BurnTicket（仅实验室）
$env:MSP_DATA = ".\msp-data" # 数据目录（默认 ./msp-data）
```

---

## 5. 5 分钟上手（单节点）

```powershell
# 0) 可选：本地信令（若不用公共种子）
.\bin\msp-seed.exe -addr 127.0.0.1:8787

# 1) 创建身份（生成 NodeID + 助记词，请离线备份）
$env:MSP_DATA = ".\msp-data-demo"
.\bin\msp.exe -c init --print-mnemonic

# 2) 查看版本 / 状态
.\bin\msp.exe -c version
.\bin\msp.exe -c identity

# 3) 配置信令种子（默认已含公共 CF）
.\bin\msp.exe -c seed list
# 本地种子示例：
.\bin\msp.exe -c seed set http://127.0.0.1:8787

# 4) 启动 mesh + 公链（不自动领币）
.\bin\msp.exe -c start
.\bin\msp.exe -c status

# 5) 本地 DTN 钱包认领（可选，+128 MST）
.\bin\msp.exe -c claim-genesis
.\bin\msp.exe -c mst

# 6) 链上认领（+128 MST，全网最多 210 名额）并出块
.\bin\msp.exe -c chain-claim
.\bin\msp.exe -c chain

# 7) 挖矿（名额用尽后主要靠这个）
.\bin\msp.exe -c chain-mine force
```

预期 `chain` 示例字段：

- `genesis_grant`: 128  
- `max_claim_nodes`: 210  
- `claimable_supply`: 26880  
- `my_balance`: 认领后常为 **178**（128 + 首块 coinbase 50，solo 矿工）  

---

## 6. 双节点互发（推荐验证路径）

### 6.1 一键冒烟

```powershell
# 终端 A：信令
.\bin\msp-seed.exe -addr 127.0.0.1:8790

# 终端 B：
$env:MSP_SEED = "http://127.0.0.1:8790"
$env:MSP_POW_FAST = "1"
.\bin\msp-p2p-test.exe
```

成功时输出类似：

```
neighbors 1 1
unicast true broadcast true
PASS full stack
```

> 注意：旧版本地 seed（`0.1.0-test` 且仍有 `packets` 字段）会与当前客户端不兼容。请使用本仓库编译的 `msp-seed`（`0.2.0-test`，signaling-only）。

### 6.2 手工双进程

**节点 A：**

```powershell
$env:MSP_DATA = ".\msp-data-a"
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c seed set http://127.0.0.1:8790
.\bin\msp.exe -c start
.\bin\msp.exe -c identity   # 记下 node_id
.\bin\msp.exe -c chain-claim
```

**节点 B：**

```powershell
$env:MSP_DATA = ".\msp-data-b"
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c seed set http://127.0.0.1:8790
.\bin\msp.exe -c start
.\bin\msp.exe -c discover
.\bin\msp.exe -c peers
.\bin\msp.exe -c chain-claim
.\bin\msp.exe -c send <A的node_id> hello-from-B
```

**节点 A 收件：**

```powershell
.\bin\msp.exe -c inbox
```

生产环境默认强制校验 BurnTicket；双节点需链同步 burn / 区块。实验室可设 `MSP_REQUIRE_TICKET=0`。

---

## 7. CLI 命令手册

统一形式：`msp -c <command> [args...]`  
无 `-c` 时启动 **GUI**（本地 HTTP + 嵌入前端）。

### 7.1 身份

| 命令 | 说明 |
|------|------|
| `init [--print-mnemonic] [--passphrase=...]` | 新建 BIP39 身份 |
| `import <助记词...>` | 导入 |
| `identity` | 显示 NodeID / 指纹 |

### 7.2 信令种子

| 命令 | 说明 |
|------|------|
| `seed list` | 列出种子 URL |
| `seed set <url>` | 设置主种子 |
| `seed add / remove <url>` | 增删 |
| `health` | 探测种子健康 |

公共种子（默认内置，以配置为准）：

- `https://msp.forbiddenx.top`  
- `https://msp-seed.tangent2533.workers.dev`  

### 7.3 Mesh / 消息

| 命令 | 说明 |
|------|------|
| `start` / `stop` | 启停 mesh + 链 |
| `status` / `discover` / `peers` | 状态与邻居 |
| `send <node_id> <text...>` | 单播（burner + ticket + DTN） |
| `broadcast <text...>` | 广播 |
| `alert <text...>` | 告警（需 Alert 公钥） |
| `inbox` / `clear-inbox` | 收件箱 |
| `block` / `unblock <id>` | 拉黑 |
| `verify-fingerprint <id> <fp16>` | 带外核对指纹 |
| `trust` | 信任存储 |

### 7.4 本地 MST 与公链

| 命令 | 说明 |
|------|------|
| `mst` | 本地钱包快照（grant=128） |
| `claim-genesis` | 本地一次性 +128（可选） |
| `chain` / `chain-status` | 高度、余额、认领名额、矿工池 |
| `chain-claim` | 链上认领 128 MST |
| `chain-mine [force]` | 出块 |
| `chain-transfer <to> <amount>` | 转账 |
| `chain-register` | 链上注册公钥 |

### 7.5 其他

| 命令 | 说明 |
|------|------|
| `version` / `-v` | 版本 |
| `block-alert` / `alert-info` | 告警密钥信息 |

---

## 8. GUI

```powershell
.\bin\msp.exe
# 或指定数据目录
$env:MSP_DATA = ".\msp-data-gui"
.\bin\msp.exe
```

浏览器访问程序打印的本地地址（嵌入 `internal/gui`）。能力与 CLI 对齐：身份、种子、邻居、收发、链状态等。

---

## 9. 自建信令种子

### 9.1 本地

```powershell
.\bin\msp-seed.exe -addr 127.0.0.1:8787
# 健康检查
curl http://127.0.0.1:8787/v1/health
```

### 9.2 Cloudflare Workers

仓库根目录：

- `_worker.js` — 信令实现（Durable Object 共享 peers）  
- `wrangler.toml` — 绑定与路由  

```bash
npx wrangler deploy
```

Free 计划 Durable Object 迁移需使用 `new_sqlite_classes`（见 `.learnings/LEARNINGS.md`）。  
大陆访问：自定义域优于 `workers.dev`，但仍受 CF 线路影响；可自建国内 `msp-seed` 做多种子故障转移。

**Tor / SOCKS：**

```powershell
$env:SOCKS5_PROXY = "socks5://127.0.0.1:9050"
# 或
$env:ALL_PROXY = "socks5://127.0.0.1:9050"
```

---

## 10. 测试

```powershell
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

主要覆盖：

- `internal/chain`：认领、burn、BurnTicket、匿名 burner、coinbase 池、210 名额上限  
- `internal/mst`：本地 128 / 4226880 常量与认领  
- `internal/pow` / `internal/dedup`  

集成：`bin/msp-p2p-test`（见 §6.1）。

---

## 11. 数据目录与安全

默认 `./msp-data`（可用 `MSP_DATA` 覆盖），典型内容：

| 路径 | 内容 |
|------|------|
| `identity.json` | 身份（可口令加密） |
| `config.json` | 种子、UDP 端口、PoW 等 |
| `trust.json` | 指纹信任 |
| `mst-ledger.json` | 本地 DTN MST |
| `chain/` | 公链区块与状态 |
| `dtn-queue.json` | 离线携带队列（若有） |

**切勿**把 `msp-data*` 提交到 Git 或发到公开频道。  
助记词只在 `init --print-mnemonic` 时出现，请离线备份；丢失无法恢复。

---

## 12. 故障排除

| 现象 | 处理 |
|------|------|
| `POST /v1/hello` 400 / 旧 seed | 换新编译的 `msp-seed` 或换端口；确认 health `version` 为 `0.2.0-test` 且 `role=signaling-only` |
| `neighbors 0` 打洞失败 | 检查防火墙 UDP、种子可达、两端 `discover`；对称 NAT 可能需中继方案（当前版本以直连打洞为主） |
| `supply exhausted` | 210 认领名额用尽 → `chain-mine` |
| `insufficient` 余额 | 先 `chain-claim` 或挖矿；发送需覆盖 burn + fee + burner 资金 |
| 收不到消息 / ticket 失败 | 确认双方链 mempool/区块同步；实验室可 `MSP_REQUIRE_TICKET=0` |
| 出块太慢 | 设 `MSP_POW_FAST=1` `MSP_CHAIN_FAST=1` |
| seed TLS 超时 | 本地 mesh/chain 仍可继续；检查代理或改本地 seed |
| 每条 CLI 状态不共享 | 每个 `msp -c` 是独立进程；需要常驻时用 GUI 或自行常驻服务 |

---

## 13. 文档地图（AI 协作痕迹保留）

| 路径 | 说明 |
|------|------|
| `Guide/guide.md` | **本文件**：使用全教程 |
| `README.md` | 项目总览 |
| `MSP白皮书.md` | 协议与威胁模型 |
| `msp-design-dialog.txt` | 设计对话记录 |
| `代办.md` … `代办7.md` | 迭代规格与审计修复清单 |
| `.learnings/LEARNINGS.md` | 踩坑与架构经验 |
| `.learnings/ERRORS.md` | 错误与修复（若存在） |
| `.claude/` | Claude 等 AI 会话/规则痕迹（**不忽略**） |

---

## 14. 版本与路线备注

- **当前**：v1.3.2-daiban7 — economy chain + BurnTicket + burner + 矿工池 B + 128×210 认领 + DTN 常量对齐 + 无自动 claim  
- **暂缓**：白皮书级 RandomX / CGO（现用 SHA-512 Hashcash 类 PoW 便于实验室）  
- **后续可增强**：多节点 ticket 声誉、远程矿工签发、多跳 burner / 环签名、header-first 同步完善等  

---

## 15. 许可

见仓库 `README.md`（当前 TBD）。使用本软件即表示你理解：这是研究/原型网络，**无主网资金保证**，请勿存入真实资产预期。

---

*Generated and maintained as part of the 100% AI-edited MSP project workflow.*
