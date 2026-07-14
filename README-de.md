# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | **Deutsch**

Zensurresistente E2EE-Nachrichten + **PoS**-Wirtschaftskette (**msp-mainnet-2**). Nachrichten über DTN/UDP; Kette nur MST-Ledger. **Kein Free-Claim**. Slot **10 Minuten**. Details: [README.md](README.md) und [MAINNET.md](MAINNET.md).

**Version:** `2.0.0-mainnet-pos` · **Chain ID:** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed manuell: `msp -c seed set <url>`. Eigener Seed: `msp-seed -addr 0.0.0.0:8787`.

## Lizenz

[MIT](LICENSE)
