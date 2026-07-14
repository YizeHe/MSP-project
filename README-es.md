# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | **Español** | [العربية](README-ar.md) | [Deutsch](README-de.md)

Mensajería E2EE resistente a la censura + cadena económica **PoS** (**msp-mainnet-2**). Mensajes por DTN/UDP; la cadena solo guarda el ledger MST. **Sin claim gratis**. Slot de **10 minutos**. Detalles: [README.md](README.md) y [MAINNET.md](MAINNET.md).

**Versión:** `2.0.0-mainnet-pos` · **Chain ID:** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed manual: `msp -c seed set <url>`. Seed propio: `msp-seed -addr 0.0.0.0:8787`.

## Licencia

[MIT](LICENSE)
