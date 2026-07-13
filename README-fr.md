# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | **Français** | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

Prototype de messagerie décentralisée, chiffrée de bout en bout et résistante à la censure. Projet **100% AI-edited**.

| Couche | Rôle |
|--------|------|
| **DTN / maillage UDP P2P** | Messages chiffrés, inondation, store-carry-forward |
| **Chaîne publique** | Registre MST + preuves de burn uniquement (pas de corps de message) |
| **Graine de signalisation** | Découverte de pairs et **hole punch** uniquement (pas de relais de chat) |

## Documentation

| Document | Langue |
|----------|--------|
| **[Guide/guide.md](Guide/guide.md)** | Anglais (par défaut) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | Chinois |
| [MSP白皮书.md](MSP白皮书.md) | Livre blanc (chinois) |

## Version

`1.3.2-daiban7` — branche de développement **`dev`**.

## Architecture en une ligne

```
Sceller le texte chiffré → burn anonyme (burner) → BurnTicket → envoi DTN
Chaîne : registre MST + hash de burn · Signalisation : hole punch uniquement
```

## Tokenomie

| Paramètre | Valeur |
|-----------|--------|
| Offre totale (sans inflation) | **4 226 880** MST |
| Réserve de claim gratuit | **26 880** (210 nœuds × **128** MST) |
| Pool des mineurs | **4 200 000** MST |

## Démarrage rapide

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

Tutoriel complet : **[Guide/guide.md](Guide/guide.md)**.

## Tests

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## Licence

[MIT](LICENSE) © 2026 YizeHe / contributeurs MSP-project.
