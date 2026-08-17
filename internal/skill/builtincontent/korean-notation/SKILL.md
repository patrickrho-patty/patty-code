---
name: korean-notation
description: 한국어 대상 프로젝트의 날짜, 시간, 통화, 전화, 주소, 우편번호 표기 지침. 코드 생성 및 리뷰 시 한국 표준 표기법을 자동 적용하기 위한 인라인 레퍼런스입니다.
runas: inline
context: codebase
auto-use: suggest
triggers:
  - 한국
  - Korea
  - 한국어
  - Korean
  - 날짜
  - 시간
  - 통화
  - 원화
  - 전화번호
  - 주소
  - 우편번호
  - frontend
  - UI
  - UX
  - locale
  - i18n
  - ko-KR
  - lang=ko
---

You are reading the Korean standard notation reference. Apply these conventions whenever you generate code, data, or user-facing text for a Korean-targeted project.

### Dates

- Default format: `YYYY. MM. DD.` (no leading zeros on month/day; space before and after each dot)
- Alternative machine-readable format: `YYYY-MM-DD`
- Prose / long form: `YYYY년 MM월 DD일`
- **Never**: `MM/DD/YYYY`, `DD/MM/YYYY`, `MM-DD-YYYY`

Reference:
- Bad:  `2024/01/05`, `01/05/2024`
- Good: `2024. 1. 5.`, `2024-01-05`, `2024년 1월 5일`

### Times

- Default: 24-hour clock (`09:30`, `14:30`, `23:59`)
- Prose / long form: `오전 H시`, `오후 H시` (Korean-style AM/PM)
- **Never**: `9:30 AM`, `2:30 PM`, `11:59 PM` in Korean-targeted UIs

Reference:
- Bad:  `2:30 PM`
- Good: `14:30`, `오후 2시 30분`

### Currency (KRW / Won)

- Symbol: `₩` (U+20A9) — never ¥, never $ for KRW amounts
- Format: `₩` prefix, thousands separator `,`, **no decimal subdivision**. Korean Won has no cent/jeon subdivision in routine usage.
- Large amounts: `₩1,000,000`

Reference:
- Bad:  `₩1000.00`, `1000원`, `$1,000`
- Good: `₩1,000`

### Phone Numbers

- Domestic: `010-XXXX-XXXX`
- International: `+82-10-XXXX-XXXX`
- Separator: `-` (hyphen), not dots or spaces
- Emergency/Public numbers: `02-XXXX-XXXX`, `031-XXX-XXXX`, `1588-XXXX`

Reference:
- Bad:  `010.XXXX.XXXX`, `010XXXXXXXX`
- Good: `010-1234-5678`, `+82-10-1234-5678`

### Addresses (도로명 주소 / Road-Name Address System)

- Order: 광역시/도 → 시/군/구 → 도로명 → 건물번호 → 상세주소 (large-to-small)
- English transliteration: Use official Korean Ministry of the Interior romanization, not ad-hoc transliteration
- **Never**: US-style small-to-large ordering (123 Main St, City, State, ZIP)

Reference:
- Bad:  `Building 302, Teheran-ro, Gangnam-gu, Seoul, 06164` (reverse order)
- Good: `서울특별시 강남구 테헤란로 302 (우) 06164`
- Good (EN): `302 Teheran-ro, Gangnam-gu, Seoul, Republic of Korea`

### Postal Codes

- Format: 5-digit basic district number (기초구역번호)
- Prefix: `(우)` or `(우) ` in prose, or bare number next to the address

Reference:
- Bad:  `061-64`, `061 64`
- Good: `06164`, `(우) 06164`

### General Rules

- **Language**: Korean user-facing text must use proper Hangul prose, not machine-translated stilted Korean
- **Right-to-left**: Korean is left-to-right; do not apply RTL layout
- **Thousands separator**: commas (`,`) per 3 digits — `₩10,000,000`
- **Date/time in file names**: use `YYYY-MM-DD` or `YYYYMMDD` for sortability
