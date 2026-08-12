# Reasoning Language

<a href="./GUIDE.md">Guide</a>
&nbsp;·&nbsp;
<a href="./GUIDE.ko-KR.md">(Korean guide)</a>

`agent.reasoning_language` controls the preferred language of visible
reasoning or thinking text when a provider exposes it.

It does not set the final answer language, rewrite code, translate identifiers,
or change hidden model reasoning. The user's explicit language request in a turn
still wins for the final answer.

## Why It Exists

Some users read visible reasoning more comfortably in Korean or English even
when the task itself mixes languages. This setting makes that preference
explicit without changing the stable system prompt or tool definitions.

The setting is intentionally small:

- `auto` anchors visible reasoning to Korean when the raw user prompt is
  clearly Han-based, ignoring injected reference context such as `@file`
  contents; English and ambiguous turns add no extra instruction.
- `ko-KR` asks visible reasoning to prefer Korean.
- `en` asks visible reasoning to prefer English.

## Desktop

Open:

```text
Settings -> Models -> Usage -> Agent runtime -> Thinking language
```

The desktop setting writes the user-level default. A project can still override
it with `./patty.toml`.

## CLI And TUI

For shell scripts or one-off configuration:

```bash
patcode config reasoning-language auto
patcode config reasoning-language ko-KR
patcode config reasoning-language en
```

By default this writes the user config. To write a project-local override:

```bash
patcode config reasoning-language --local ko-KR
```

Inside `patcode`, use the slash command:

```text
/reasoning-language auto
/reasoning-language ko-KR
/reasoning-language en
```

The slash command writes the user-level setting and updates the current chat
controller for subsequent turns. It does not rewrite the current project's
`patty.toml`; use the shell command with `--local` for that.

Headless runs also use the same setting:

```bash
patcode run "explain this module"
```

## Config File

User or project config:

```toml
[agent]
reasoning_language = "auto" # auto|ko-KR|en
```

Resolution order for this setting:

```text
./patty.toml > user config.toml > built-in defaults
```

There is currently no command-line flag for this setting. Prefer config because
the value is a user or project preference rather than a per-invocation task
argument.

## Cache Behavior

`auto` is still cache-friendly. When the raw user prompt clearly looks Han-based,
Patty Code adds the same small transient `<reasoning-language>` block for that
turn; English and ambiguous turns inject nothing and rely on the existing stable
language policy. Injected reference context such as `@file` contents is ignored
for this auto decision.

When set to `ko-KR` or `en`, Patty Code always adds a small transient
`<reasoning-language>` block to the user turn. In all modes, this does not
change:

- the system prompt
- tool schema bytes or ordering
- the stable provider-visible prefix

This keeps high prompt-cache hit rate intact while still letting an explicit
preference affect the next model call.

## Boundaries

- The setting only matters when visible reasoning text exists.
- It is a preference, not a hard translation layer.
- Code, identifiers, file paths, shell commands, and untranslated technical
  terms should remain in their original form.
- If a user asks for a final answer in a specific language, that request remains
  authoritative for the final answer.
- The visible-reasoning language is anchored mainly by two signals: the language
  of the turn's first reasoning segment, and the language of earlier reasoning
  segments that providers receive back during tool-call loops. The injected
  language block works by landing the first segment in the preferred language;
  once the first segment holds, later segments usually sustain it.
- In long agent turns dominated by another language (for example large English
  build logs, code, or tool output), a later reasoning segment can still drift;
  once it drifts, the rest of the turn usually stays in the drifted language,
  and restating the preference mid-turn recovers it only partially. This is a
  model-behavior boundary; the setting stays best-effort by design.
