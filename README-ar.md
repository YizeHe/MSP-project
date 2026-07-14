# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | **العربية** | [Deutsch](README-de.md)

مراسلة مشفّرة مقاومة للرقابة + سلسلة اقتصادية **PoS** (**msp-mainnet-2**). الرسائل عبر DTN/UDP؛ السلسلة لدفتر MST فقط. **بدون claim مجاني**. الفترة **10 دقائق**. التفاصيل: [README.md](README.md) و [MAINNET.md](MAINNET.md).

**الإصدار:** `2.0.0-mainnet-pos` · **Chain ID:** `msp-mainnet-2`

```bash
git checkout dev
go build -o bin/msp ./cmd/msp
export MSP_DATA=./msp-mainnet-data-v2
./bin/msp -c init && ./bin/msp -c mine
```

Seed يدوي: `msp -c seed set <url>`. Seed خاص: `msp-seed -addr 0.0.0.0:8787`.

## الترخيص

[MIT](LICENSE)
