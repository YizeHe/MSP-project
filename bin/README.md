# bin/ — 本地可执行文件目录

本目录用于存放 **本机编译** 的二进制，**不提交到 Git**（见根目录 `.gitignore`）。

## 构建（Windows / PowerShell）

在仓库根目录执行：

```powershell
$env:CGO_ENABLED = "0"
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
go build -o bin/msp-p2p-test.exe ./cmd/msp-p2p-test
```

## 构建（Linux / macOS）

```bash
go build -o bin/msp ./cmd/msp
go build -o bin/msp-seed ./cmd/msp-seed
go build -o bin/msp-p2p-test ./cmd/msp-p2p-test
```

## 产物说明

| 文件 | 用途 |
|------|------|
| `msp` / `msp.exe` | 主客户端：CLI（`-c`）与 GUI |
| `msp-seed` / `msp-seed.exe` | 本地信令种子（打洞，不中继消息） |
| `msp-p2p-test` / `msp-p2p-test.exe` | 双节点 P2P 冒烟测试 |

## Docs

- English (default): **[../Guide/guide.md](../Guide/guide.md)**  
- 中文: **[../Guide/guide-cn.md](../Guide/guide-cn.md)**  
- License: **[MIT](../LICENSE)**
