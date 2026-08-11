# Patty Medium Default Design

## Goal

Make a stock `patcode` installation use the Patty Omni combo model `medium` exclusively, with the requested context and automatic-compaction boundaries.

## Runtime contract

- Provider name: `patty`
- Protocol: OpenAI-compatible Chat Completions
- Base URL: `https://omni.agents.patty.io/v1`
- Model ID and visible model label: `medium`
- Credential environment variable: `AGENTS_PATTY_API_KEY`
- Context window: `248124` tokens
- Automatic-compaction threshold: `238123` tokens
- Force-compaction ceiling: 98% of the context window

The authenticated `/v1/models` catalog advertises `medium`. A live Chat Completions request accepts the combo, reaches its selected upstream, and returns an authenticated HTTP 200 streaming response.

## Configuration design

`config.Default()` will contain one provider, `patty/medium`, instead of the two legacy DeepSeek defaults. The compact threshold remains represented through the existing agent ratio contract, using the exact ratio `238123 / 248124`; this avoids introducing a parallel compaction setting. The configured force ratio becomes `0.98`, which remains above the requested automatic threshold.

The compact-ratio editor range expands from 65–85% to 65–97% so the shipped default remains a valid editable value. Existing lower-ratio user choices remain valid.

Existing customized user configurations are not silently migrated. This workspace's current stock-style `~/.patty/config.toml` will be updated explicitly because the user requested the new active default and a one-entry `/model` catalog.

Patcode's runtime resolver reads provider credentials through its existing secure credential store. For this workspace, the value is sourced from `AGENTS_PATTY_API_KEY` in the shell startup file and imported through that existing store path; it is never written to repository or plaintext config files.

Desktop onboarding validates the stock `medium` catalog before persisting the
credential. Official-provider activation holds the user-config lock before the
credential lock across validation and both writes, repairs required Patty wire
fields, and preserves safe request customizations such as headers and extra body.

## TUI behavior

Boot already labels a session from the resolved model ID, so resolving `patty/medium` naturally displays `medium` in the top status bar. The `/model` picker will render the model portion (`medium`) as its visible label while retaining the canonical `patty/medium` ID for switching and persistence.

## Verification

- Config tests pin the one-provider default, endpoint, protocol, credential name, context window, and exact derived compaction threshold.
- CLI tests pin the single configured model reference and visible picker label.
- Compact-ratio tests pin acceptance of the new default and rejection above the supported ceiling.
- Build a fresh signed `bin/patcode`, launch it in a PTY, and inspect the captured TUI for the `medium` header and single model picker entry.
- Run full Go, desktop, frontend, and site gates plus CGO-free Linux and Windows cross-builds.
- Run a live Patty Omni smoke request separately; report upstream health independently from local correctness.
