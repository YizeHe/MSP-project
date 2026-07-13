# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | **Español** | [العربية](README-ar.md) | [Deutsch](README-de.md)

Prototipo de mensajería descentralizada con cifrado de extremo a extremo y resistencia a la censura. Proyecto **100% AI-edited**.

| Capa | Función |
|------|---------|
| **DTN / malla UDP P2P** | Mensajes cifrados, inundación, store-carry-forward |
| **Cadena pública** | Solo libro MST y pruebas de burn (sin cuerpos de mensaje) |
| **Semilla de señalización** | Descubrimiento de pares y **hole punch** (sin retransmisión de chat) |

## Documentación

| Documento | Idioma |
|-----------|--------|
| **[Guide/guide.md](Guide/guide.md)** | Inglés (predeterminado) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | Chino |
| [MSP白皮书.md](MSP白皮书.md) | Libro blanco (chino) |

## Versión

`1.3.2-daiban7` — rama de desarrollo **`dev`**.

## Arquitectura en una línea

```
Sellar ciphertext → burn anónimo (burner) → BurnTicket → envío DTN
Cadena: libro MST + hash de burn · Señalización: solo hole punch
```

## Tokenómica

| Parámetro | Valor |
|-----------|-------|
| Suministro total (sin inflación) | **4.226.880** MST |
| Reserva de claim gratuito | **26.880** (210 nodos × **128** MST) |
| Pool de recompensas de mineros | **4.200.000** MST |

## Inicio rápido

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

Tutorial completo: **[Guide/guide.md](Guide/guide.md)**.

## Pruebas

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## Licencia

[MIT](LICENSE) © 2026 YizeHe / colaboradores de MSP-project.
