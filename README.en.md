<p align="center">
  <img src="docs/logo.svg" alt="Patty Code — Korean-first coding agent" width="680"/>
</p>

<p align="center">
  <a href="./README.md">한국어</a>
  &nbsp;·&nbsp;
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="https://code.patty.io">Website</a>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.md">Guide</a>
  &nbsp;·&nbsp;
  <a href="https://github.com/patrickrho-patty/patty-code/releases">Releases</a>
</p>

<p align="center">
  <a href="https://github.com/patrickrho-patty/patty-code/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/patrickrho-patty/patty-code/ci.yml?style=flat-square&label=CI&labelColor=111827" alt="CI status"/></a>
  <a href="https://www.npmjs.com/package/patty-code"><img src="https://img.shields.io/npm/v/patty-code.svg?style=flat-square&label=npm&labelColor=111827" alt="npm version"/></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-d6a84b.svg?style=flat-square&labelColor=111827" alt="MIT license"/></a>
</p>

<h3 align="center">Think in Korean. Finish the work in your terminal.</h3>

<p align="center">
  Patty Code is an open-source coding-agent harness designed with Korean as its default language.<br/>
  It combines a deliberate TUI, controllable autonomy, and replaceable models and tools in one local runtime.
</p>

```text
╭─ Patty Code ────────────────────────────────────────────────────╮
│ [ WORK auto ] [ MODEL medium ] [ REASONING auto ] [ FREE 100% ] │
╰─────────────────────────────────────────────────────────────────╯

             TAEGEUKGI  ·  PATTY WORKFLOW

╭─ MESSAGE INPUT ────────────────────────────────────────────────╮
│   Type a command or question                                   │
╰────────────────────────────────────────────────────────────────╯
             READY · Shift+Tab GENERAL/AUTO/PLAN · Ctrl+Y YOLO
```

## What makes Patty Code different

- **Korean is the default, not an add-on.** The TUI, help, errors, setup flow, and built-in commands are written to feel natural in Korean first.
- **Korean input is treated as product behavior.** CJK grapheme boundaries, IME composition, cursor movement, and deletion have dedicated compatibility paths and regression coverage.
- **The terminal interface has its own identity.** A title bar, clearly bounded status indicators, centered Taegeukgi and Patty marks, a rounded composer, natural startup height, and semantic color themes form one coherent shell.
- **Korean commands are quick to reach.** The palette searches Korean command names, English names, and initial-consonant (초성) aliases without executing ambiguous input.
- **You choose the autonomy boundary.** General, Auto, and Plan modes work with permission rules, sandboxing, checkpoints, rewind, and session recovery to keep long runs under control.
- **The harness can outlive any one model.** Providers, MCP servers, Skills, plugins, subagents, and ACP clients connect to the same runtime.
- **Capabilities arrive through a marketplace.** Browse and install packages from the plugin marketplace, including the HWPX plugin for reading and analyzing Korean documents inside the same workflow.

## Quick start

### Install with npm

Node.js 18 or newer is required. Installation downloads the native `patcode` binary for your operating system and architecture.

```sh
npm i -g patty-code
patcode setup
patcode
```

### Use a release binary

[GitHub Releases](https://github.com/patrickrho-patty/patty-code/releases) provides archives and checksums for macOS, Linux, and Windows.

### Build from source

```sh
git clone https://github.com/patrickrho-patty/patty-code.git
cd patty-code
make build
./bin/patcode setup
./bin/patcode
```

`make cross` produces six CLI builds under `dist/`: `darwin|linux|windows × amd64|arm64`.

## Stock runtime

A new installation starts with one Patty-managed model configuration.

| Setting | Default |
| --- | --- |
| Model reference | `patty/medium` |
| Display name | `medium` |
| API | `https://omni.agents.patty.io/v1` |
| Context window | `248124` tokens |
| Automatic compaction | `238123` tokens (`95.96935403266109%`) |
| Forced compaction | `98%` |
| Credential name | `AGENTS_PATTY_API_KEY` |

`patcode setup` stores the key in Patty Code's credential store. The stock model picker contains only `medium`; you can explicitly add other OpenAI-compatible providers when needed.

## Ways to run it

```sh
patcode                                         # interactive TUI
patcode -p "summarize this repository"          # print one result
patcode run "find the cause of the failing test" # headless task
patcode run --auto "fix and verify the issue"    # explicitly allow unattended writes
patcode acp                                     # connect an ACP host or editor
patcode serve                                   # start the HTTP + SSE server
```

The interactive TUI exposes Korean commands such as `/모델전환`, `/작업모드`, `/테마전환`, `/언어설정`, and `/도움말`, alongside their English names. Type `/` for the command palette, `@` for files, or `!` for shell input.

## One runtime, multiple surfaces

| Surface | Use it for |
| --- | --- |
| CLI / TUI | Conversational repository work with visible tool approvals |
| Headless | Scripts, CI, and repeatable automation through `run` and `-p` |
| Desktop | A graphical workspace and settings UI backed by the local runtime |
| ACP | Compatible editors and hosts that drive Patty Code sessions over stdio |
| Serve | Browser and remote clients connecting over HTTP + SSE |

Every surface shares the same provider configuration, permission model, session history, and extension system.

## Plugin marketplace and HWPX

A plugin package can add Skills, agents, slash commands, hooks, MCP tools, and themes as one installable unit. The marketplace shows package compatibility and included capabilities before installation; installed packages can be disabled, re-enabled, and diagnosed.

The HWPX plugin is an official Patty Code extension available through the marketplace. It turns `.hwpx` documents into context Patty Code can read and analyze, keeping Korean document work inside the agent instead of a separate conversion flow.

```sh
patcode plugin list
patcode plugin show <name>
patcode plugin install <source> --dry-run
patcode plugin install <source> --yes
patcode plugin disable <name>
patcode plugin enable <name>
patcode plugin doctor <name>
```

Use `/플러그인` in the interactive TUI or open the plugin marketplace in Desktop to manage the same installed state.

## Themes

Keep the interface structure while changing its palette. Configure the theme in `~/.patty/config.toml`, or override it for one run through the environment.

```toml
[ui]
theme = "auto"                 # auto | dark | light
theme_style = "seoul-night"    # seoul-night | ink-night | hanji-light | jade-night
```

## Documentation

- [Guide](./docs/GUIDE.md)
- [CLI reference](./docs/CLI.md)
- [Configuration and credential paths](./docs/CONFIG_PATHS.md)
- [ACP editor integration](./docs/ACP.md)
- [Extensions and plugins](./docs/EXTENSIONS.md)
- [Product specification](./docs/SPEC.md)
- [Migration guide](./docs/MIGRATING.md)

## Development

```sh
make test
make vet
make cross
```

Read [CONTRIBUTING.md](./CONTRIBUTING.md) before proposing a change. Report security issues through the process in [SECURITY.md](./SECURITY.md).

## License

Patty Code is released under the [MIT License](./LICENSE).

<p align="center">
  <strong>Patty Code</strong><br/>
  <sub>Korean-first · terminal-native · user-controlled agency</sub>
</p>
