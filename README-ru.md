# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | **Русский** | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

Прототип децентрализованного обмена сообщениями с сквозным шифрованием и устойчивостью к цензуре. Проект **100% AI-edited**.

| Слой | Назначение |
|------|------------|
| **DTN / UDP P2P mesh** | Шифрованные сообщения, flood, store-carry-forward |
| **Публичный блокчейн** | Только реестр MST и burn-доказательства (без тел сообщений) |
| **Сигналинг-сид** | Обнаружение пиров и **hole punch** (без ретрансляции чата) |

## Документация

| Документ | Язык |
|----------|------|
| **[Guide/guide.md](Guide/guide.md)** | Английский (по умолчанию) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | Китайский |
| [MSP白皮书.md](MSP白皮书.md) | Белая книга (китайский) |

## Версия

`1.3.2-daiban7` — ветка разработки **`dev`**.

## Архитектура в одну строку

```
Запечатать ciphertext → анонимный burn (burner) → BurnTicket → отправка DTN
Цепь: реестр MST + hash burn · Сигналинг: только hole punch
```

## Токеномика

| Параметр | Значение |
|----------|----------|
| Общая эмиссия (без инфляции) | **4 226 880** MST |
| Пул бесплатных claim | **26 880** (210 узлов × **128** MST) |
| Пул наград майнеров | **4 200 000** MST |

## Быстрый старт

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

Полное руководство: **[Guide/guide.md](Guide/guide.md)**.

## Тесты

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## Лицензия

[MIT](LICENSE) © 2026 YizeHe / участники MSP-project.
