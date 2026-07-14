# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | **Français** | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

Messagerie E2EE résistante à la censure + chaîne économique **PoS** (**msp-mainnet-2**). Messages en DTN/UDP ; la chaîne ne stocke que le ledger MST. **Pas de claim gratuit**. Slot **10 minutes**. Détails : [README.md](README.md) (EN) et [MAINNET.md](MAINNET.md).

**Version :** `2.0.0-mainnet-pos` · **Chain ID :** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed manuel : `msp -c seed set <url>`. Seed auto-hébergé : `msp-seed -addr 0.0.0.0:8787`.

## Licence

[MIT](LICENSE)
