# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

[English](README.md) | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | **العربية** | [Deutsch](README-de.md)

نموذج أولي لشبكة مراسلة لا مركزية مشفّرة من طرف إلى طرف ومقاومة للرقابة. المشروع **100% AI-edited**.

| الطبقة | الدور |
|--------|-------|
| **DTN / شبكة UDP P2P** | رسائل مشفّرة، فيضان، تخزين ونقل لاحق |
| **السلسلة العامة** | دفتر MST وإثباتات الحرق فقط (بدون محتوى الرسائل) |
| **بذرة الإشارة** | اكتشاف الأقران و**ثقب NAT** فقط (بدون ترحيل الدردشة) |

## الوثائق

| المستند | اللغة |
|---------|-------|
| **[Guide/guide.md](Guide/guide.md)** | الإنجليزية (الافتراضية) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | الصينية |
| [MSP白皮书.md](MSP白皮书.md) | الورقة البيضاء (صينية) |

## الإصدار

`1.3.2-daiban7` — فرع التطوير **`dev`**.

## البنية في سطر واحد

```
ختم النص المشفّر → حرق مجهول (burner) → BurnTicket → إرسال DTN
السلسلة: دفتر MST + تجزئة الحرق · الإشارة: ثقب NAT فقط
```

## اقتصاد الرمز

| المعامل | القيمة |
|---------|--------|
| إجمالي العرض (بدون تضخم) | **4,226,880** MST |
| حوض المطالبة المجانية | **26,880** (210 عقدة × **128** MST) |
| حوض مكافآت المعدّنين | **4,200,000** MST |

## بداية سريعة

```powershell
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
```

الدليل الكامل: **[Guide/guide.md](Guide/guide.md)**.

## الاختبارات

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## الترخيص

[MIT](LICENSE) © 2026 YizeHe / مساهمو MSP-project.
