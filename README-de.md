# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | **Deutsch**

Zensurresistentes, Ende-zu-Ende-verschlüsseltes Messaging-Netzwerk (Prototyp). Projekt **100% AI-edited**.

| Schicht | Aufgabe |
|---------|---------|
| **DTN / UDP-P2P-Mesh** | Verschlüsselte Nachrichten, Flood, Store-Carry-Forward |
| **Öffentliche Chain** | Nur MST-Ledger und Burn-Nachweise (keine Nachrichteninhalte) |
| **Signaling-Seed** | Peer-Discovery und **Hole Punch** (kein Chat-Relay) |

## Dokumentation

| Dokument | Sprache |
|----------|---------|
| **[Guide/guide.md](Guide/guide.md)** | Englisch (Standard) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | Chinesisch |
| [MSP白皮书.md](MSP白皮书.md) | Whitepaper (Chinesisch) |

## Version

`1.3.2-daiban7` — Entwicklungsbranch **`dev`**.

## Architektur in einer Zeile

```
Ciphertext siegeln → anonymer Burner-Burn → BurnTicket → DTN-Versand
Chain: MST-Ledger + Burn-Hash · Signaling: nur Hole Punch
```

## Tokenomics

| Parameter | Wert |
|-----------|------|
| Gesamtangebot (keine Inflation) | **4.226.880** MST |
| Free-Claim-Pool | **26.880** (210 Knoten × **128** MST) |
| Miner-Belohnungspool | **4.200.000** MST |

## Schnellstart

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

Vollständige Anleitung: **[Guide/guide.md](Guide/guide.md)**.

## Tests

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## Lizenz

[MIT](LICENSE) © 2026 YizeHe / MSP-project-Mitwirkende.
