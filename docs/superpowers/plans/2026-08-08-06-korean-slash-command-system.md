# Plan 06: Korean Slash Command System

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 05 complete; internal/command registry exists  
**Gates:** G5 Command UX  

## 1. Purpose

Replace English slash command names with Korean canonical names, add full 초성 aliases, implement keyword-based resolution, and ensure ambiguity never auto-executes.

## 2. Scope

- Built-in command registry overhaul
- `/session.resume` → `/이어하기` (초성: `/ㅇㅇㅎㄱ`)
- Korean/English keyword search in palette
- Stable internal IDs preserved across locale changes

### Exclusions
- User-created commands (keep declared names)
- External tool/command plugins

## 3. Task List

### T1: Define stable command ID system
- Every command gets a unique, locale-independent internal ID
- Example: `session.resume` always resolves regardless of display name
- Registry maps: `{ko_name, ko_keywords, en_name, en_keywords, chosung} → internalID`

### T2: Create Korean slash command catalog for all built-ins
- Audit current 60+ built-in commands
- Assign Korean display name + 초성 alias to each
- Create English fallbacks

### T3: Implement multi-keyword resolver
- Exact canonical match > exact alias match > keyword match
- 초성 input must not auto-execute if ambiguous
- Keyword ranking deterministic by frequency and relevance scores

### T4: Update CLI completion
- Tab completion uses Korean names when locale=ko
- Keyboard shortcuts preserved across locales

### T5: Update desktop completion/help
- Same resolution engine as CLI
- Help displays Korean descriptions with English parenthetical

### T6: Add comprehensive tests
- Every command resolves correctly by all input types
- Ambiguity test: overlapping 초성 doesn't execute
- Empty palette state when no matches

## 4. Definition of Done

- [ ] All built-in commands have Korean canonical names
- [ ] Each has working 초성 alias
- [ ] English keywords still resolve when active
- [ ] No ambiguous 초ectution
- [ ] Gate G5 proof: every built-in resolves by Korean, 초성, and keywords without ambiguous execution