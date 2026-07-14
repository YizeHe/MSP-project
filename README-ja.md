# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | **日本語** | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

検閲耐性のある E2EE メッセージング + **PoS 経済チェーン**（**msp-mainnet-2**）。メッセージは DTN/UDP、チェーンは MST 台帳のみ。無料クレームなし。スロット **10 分**。詳細は英語 [README.md](README.md) と [MAINNET.md](MAINNET.md)。

**Version:** `2.0.0-mainnet-pos` · **Chain ID:** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed は手動設定可: `msp -c seed set <url>`。自前シード: `msp-seed -addr 0.0.0.0:8787`。

## License

[MIT](LICENSE)
