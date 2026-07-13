# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | **日本語** | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

検閲耐性のあるエンドツーエンド暗号化メッセージングのプロトタイプです。コードとドキュメントは **100% AI-edited** です。

| 層 | 役割 |
|----|------|
| **DTN / UDP P2P mesh** | 暗号化メッセージ、フラッド、ストア＆フォワード |
| **パブリックチェーン** | MST 台帳と burn 証明のみ（本文なし） |
| **シグナリングシード** | ピア発見と **ホールパンチ** のみ（チャット中継なし） |

## ドキュメント

| 文書 | 言語 |
|------|------|
| **[Guide/guide.md](Guide/guide.md)** | 英語（デフォルト） |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | 中国語 |
| [MSP白皮书.md](MSP白皮书.md) | ホワイトペーパー（中国語） |

## バージョン

`1.3.2-daiban7` — 開発は **`dev`** ブランチを推奨。

## アーキテクチャ（一行）

```
暗号文を密封 → burner で匿名 burn → BurnTicket → DTN 送信
チェーン: MST 台帳 + burn ハッシュ · シグナリング: ホールパンチのみ
```

## トークン経済

| 項目 | 値 |
|------|-----|
| 総供給（インフレなし） | **4,226,880** MST |
| 無料クレーム枠 | **26,880**（210 ノード × **128** MST） |
| マイナー報酬プール | **4,200,000** MST |

## クイックスタート

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

詳細手順: **[Guide/guide.md](Guide/guide.md)**。

## テスト

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## ライセンス

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors。
