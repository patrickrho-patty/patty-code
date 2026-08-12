# Patty Code documentation

This directory contains the documentation that ships with Patty Code and the
maintainer material used to build and release it.

## Start here

- [Guide](guides/GUIDE.md) · [한국어 가이드](guides/GUIDE.ko-KR.md) — everyday setup and usage
- [CLI reference](guides/CLI.md) — commands, automation, and output formats
- [Configuration paths](guides/CONFIG_PATHS.md) — config, credentials, state, and migration
- [Migrating to Patty Code](guides/MIGRATING.md) — moving from the legacy line

## Product and runtime reference

- [Engineering spec](reference/SPEC.md)
- [ACP editor integration](guides/ACP.md)
- [Recovery and diagnostics](guides/RECOVERY.md)
- [Checkpoints and rewind](reference/CHECKPOINTS.md)
- [Reasoning language](reference/REASONING_LANGUAGE.md)
- [Reasoning providers](reference/REASONING_PROVIDERS.md)
- [Context Engine retrieval](reference/SESSION_MEMORY_RETRIEVAL.md)
- [Task contracts](reference/TASK_CONTRACT.md)
- [Tool contract](reference/TOOL_CONTRACT.md)
- [Tool approval modes](reference/TOOL_APPROVAL_MODES.md) · [한국어](reference/TOOL_APPROVAL_MODES.ko-KR.md)

## Extensions, plugins, and themes

- [Extensions](extensions/EXTENSIONS.md) · [한국어](extensions/EXTENSIONS.ko-KR.md)
- [Plugin packages](extensions/PLUGIN_PACKAGES.md) · [한국어](extensions/PLUGIN_PACKAGES.ko-KR.md)
- [Extension Protocol](extensions/EXTENSION_PROTOCOL.md) · [한국어](extensions/EXTENSION_PROTOCOL.ko-KR.md)
- [Generated protocol index](extensions/EXTENSION_PROTOCOL.generated.md) — generated; do not edit
- [Extension Runtime v2](extensions/EXTENSION_RUNTIME_V2.md)
- [Extension Runtime v2 performance](extensions/EXTENSION_RUNTIME_V2_PERF.md)
- [Theme Pack](themes/THEME_PACK.md)
- [Theme asset provenance](themes/THEME_ASSETS.md)
- [Capability diagnostics](reference/CAPABILITY_DIAGNOSTICS.md)

## Integrations and operations

- [Bot guide](guides/BOT_GUIDE.md)
- [Subagent profiles](reference/SUBAGENT_PROFILES.md)
- [Releasing](operations/RELEASING.md)
- [Windows SignPath SOP](operations/SIGNPATH_WINDOWS_ADMIN_SOP.md)

## Maintainer material

- [`assets/`](assets/) contains documentation artwork and design references.
- [`archive/`](archive/) contains retired, unreferenced material kept only for
  provenance. It is not part of the shipped `/docs` corpus.

Private planning notes belong in the locally ignored `docs/superpowers/`
directory and are intentionally not part of public source or the shipped docs.

The Go build recursively embeds the categorized Markdown trees into the exact
binary that users run. Moving a public guide changes its runtime path and must
be treated as a product/API change; the canonical paths listed above are the
ones returned by `/docs`.

English/Korean pairs are intentionally separate files so `/docs` can select a
locale without duplicating or merging large documents at runtime. The generated
protocol index is also intentionally separate from its prose companion because
the generator and CI own that file.
