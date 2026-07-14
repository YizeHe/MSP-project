# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | **Русский** | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

Цензуроустойчивый E2EE-мессенджер + экономическая цепь **PoS** (**msp-mainnet-2**). Сообщения — DTN/UDP; в цепи только реестр MST. **Без бесплатного claim**. Слот **10 минут**. Подробности: [README.md](README.md) и [MAINNET.md](MAINNET.md).

**Версия:** `2.0.0-mainnet-pos` · **Chain ID:** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed вручную: `msp -c seed set <url>`. Свой seed: `msp-seed -addr 0.0.0.0:8787`.

## Лицензия

[MIT](LICENSE)
