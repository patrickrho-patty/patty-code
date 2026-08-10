<p align="center">
  <img src="docs/logo-ghost-wave-effect.svg" alt="Patty Code" width="360"/>
</p>

<p align="center">
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="./README.ko-KR.md">한국어</a>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.md">Guide</a>
  &nbsp;·&nbsp;
  <a href="./docs/ACP.md">ACP</a>
  &nbsp;·&nbsp;
  <a href="./docs/EXTENSIONS.md">Extensions</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">Spec</a>
  &nbsp;·&nbsp;
  <a href="https://pattycorp.github.io/DeepSeek-PattyCode/">Website</a>
  &nbsp;·&nbsp;
  <strong><a href="https://discord.gg/XF78rEME2D">Discord</a></strong>
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/patty"><img src="https://img.shields.io/npm/v/patty-code.svg?style=flat-square&color=cb3837&labelColor=161b22&logo=npm&logoColor=white" alt="npm version"/></a>
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/pattycorp/DeepSeek-PattyCode/ci.yml?style=flat-square&label=ci&labelColor=161b22&logo=githubactions&logoColor=white" alt="CI"/></a>
  <a href="./LICENSE"><img src="https://img.shields.io/npm/l/patty-code.svg?style=flat-square&color=8b949e&labelColor=161b22" alt="license"/></a>
  <a href="https://www.npmjs.com/package/patty"><img src="https://img.shields.io/npm/dm/patty-code.svg?style=flat-square&color=3fb950&labelColor=161b22&label=downloads" alt="downloads"/></a>
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/stargazers"><img src="https://img.shields.io/github/stars/pattycorp/DeepSeek-PattyCode.svg?style=flat-square&color=dbab09&labelColor=161b22&logo=github&logoColor=white" alt="GitHub stars"/></a>
  <a href="https://atomgit.com/pattycorp/DeepSeek-PattyCode"><img src="https://atomgit.com/pattycorp/DeepSeek-PattyCode/star/badge.svg" alt="AtomGit stars"/></a>
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors"><img src="https://img.shields.io/github/contributors/pattycorp/DeepSeek-PattyCode.svg?style=flat-square&color=bc8cff&labelColor=161b22&logo=github&logoColor=white" alt="contributors"/></a>
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/discussions"><img src="https://img.shields.io/github/discussions/pattycorp/DeepSeek-PattyCode.svg?style=flat-square&color=58a6ff&labelColor=161b22&logo=github&logoColor=white" alt="Discussions"/></a>
  <a href="https://discord.gg/XF78rEME2D"><img src="https://img.shields.io/badge/discord-join-5865F2.svg?style=flat-square&labelColor=161b22&logo=discord&logoColor=white" alt="Discord"/></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/27020?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-27020" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/27020/monthly?language=Go" alt="Patty Code | Trendshift" width="250" height="55"/></a>
  <a href="https://trendshift.io/repositories/27020?utm_source=repository-badge&amp;utm_medium=badge&amp;utm_campaign=badge-repository-27020" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/repositories/27020" alt="Patty Code | Trendshift" width="250" height="55"/></a>
</p>

<br/>

<p align="center"><strong>Open source · MIT · Korea's #1 Korean-first commercial coding agent harness</strong></p>
<h3 align="center">Patty Code is built for Korean developers first.</h3>
<p align="center">Patty Code, from <strong>Patty Co., Ltd.</strong> (<strong>주식회사 패티</strong>), is the first commercial Korean-specific coding agent harness: one local engine with terminal, desktop, browser, and ACP editor entry points, built around Korean workflows from prompt to approval. It ships Korean-first command UX, Korean reasoning support, HWPX parsing, IME-safe composition, caret-safe input, 초성 slash commands, and the long-run controls you need to trust an autonomous agent.</p>

<div align="center">
  <video src="https://github.com/user-attachments/assets/ab2f3878-e224-4931-8254-060e7695cfb9" controls preload="metadata" width="560"></video>
</div>

<br/>

> [!IMPORTANT]
> **Community** — Discord for setup help, Korean workflow showcases, and feature ideas. → **<https://discord.gg/XF78rEME2D>**

<br/>

## Why Patty Code

- **Korean-first by design.** Patty Code treats Korean as a first-class product surface, not a bolt-on translation layer. The runtime, docs, reasoning-language controls, and desktop UX already understand `ko-KR` as a core mode.
- **Korean input that does not fight the user.** The desktop composer preserves IME composition state, keeps Enter from breaking composition confirmation, and restores the caret correctly after Korean input commits.
- **초성-native command UX.** Built-in slash commands resolve through Korean canonical names, 초성 aliases, and English names, with ambiguity-safe behavior so fuzzy Korean command input still feels reliable.
- **Korean document workflows.** Patty Code supports HWPX parsing and is designed for real Korean document-heavy workflows, not just plain English code prompts.
- **Mixed-language safe.** Korean, English, and other double-width CJK text are handled cleanly in mixed-language input, terminal rendering, and command interaction.
- **Autonomy you can audit.** Plan mode, tool approvals, workspace sandboxing, rewind, checkpoints, branches, and session history make long autonomous runs inspectable and reversible.
- **Commercial-grade harness, open kernel.** The project stays MIT and developer-friendly while giving teams a production-minded harness around models, tools, policy, recovery, and UX.
- **Multi-provider and composable.** DeepSeek is one preset, not the whole product. Any OpenAI-compatible endpoint can be configured, and dual-model executor/planner setups remain first-class.

## Core features

- **Config-driven.** Providers, the agent, enabled tools, and plugins are all declared in `patty.toml`.
- **Desktop + CLI + browser + ACP.** One local Patty Code engine, four ways in.
- **Plugin-driven.** MCP servers and Extension Protocol sidecars can contribute tools, prompts, providers, resources, and structured UI.
- **Cache-aware context maintenance.** Startup injects a stable environment summary, stale tool output is pruned before compaction, and the tool schema contract is documented for regression review.
- **Zero-friction distribution.** `CGO_ENABLED=0` single binary; cross-compile to six targets with one command.

## Install

Choose the path that matches how you want to use Patty Code. The CLI/TUI, desktop app, and VS Code extension all use the same local Patty Code engine.

### Path A: CLI / TUI

Install the native binary through npm on any supported platform, or use Homebrew on macOS:

```sh
npm i -g patty code                  # any OS; pulls the prebuilt native binary
brew install pattycorp/patty/patty code   # macOS
```

Prebuilt archives (`darwin|linux|windows × amd64|arm64`) and `SHA256SUMS` are on every [GitHub release](https://github.com/pattycorp/DeepSeek-PattyCode/releases).

### Path B: Desktop app

Use the [official download page](https://patty-code.io/?download=desktop#start) for the latest desktop build.

| Platform | Package | Architecture |
| --- | --- | --- |
| macOS | Universal `.dmg` or `.zip` | Apple Silicon / Intel |
| Windows | Installer `.exe` or portable `.zip` | x64 / ARM64 |
| Linux | `.deb` or `.tar.gz` | x64 |

Windows installers are code-signed through [SignPath.io](https://signpath.io/) with a free certificate provided by the [SignPath Foundation](https://signpath.org/).

### Path C: VS Code extension

Complete Path A first. The extension does not bundle the CLI; it starts your local `patty code acp` backend and adds native chat, editor context, tool-call approvals, model selection, and workspace sessions.

- **VS Code:** [install from Visual Studio Marketplace](https://marketplace.visualstudio.com/items?itemName=SivanLiu.patty-agent)
- **VSCodium / Eclipse Theia:** [install from Open VSX Registry](https://open-vsx.org/extension/SivanLiu/patty-code-agent)
- **Extension ID:** `SivanLiu.patty-agent` · [source and usage guide](https://github.com/SivanCola/patty-code-vscode)

### Path D: Build from source

```sh
git clone https://github.com/pattycorp/DeepSeek-PattyCode.git
cd DeepSeek-PattyCode
make build      # -> bin/patty(.exe)
make cross      # -> dist/ (darwin|linux|windows × amd64|arm64)
```

## Quick start

### CLI / TUI

These commands are for the CLI/TUI installed through Path A:

```sh
patty code setup
patty code
patty code run "implement the TODOs in main.go"
```

In an interactive session, run `/init` when you want Patty Code to create project instructions.

### Desktop app

Download the installer for your platform from the [official download page](https://patty-code.io/?download=desktop#start), install and launch Patty Code, then configure a provider and model in the app. The CLI commands above are not required for the desktop app.

For advanced CLI usage and configuration, see the **[CLI reference](./docs/CLI.md)**, **[Guide](./docs/GUIDE.md)**, and **[configuration paths](./docs/CONFIG_PATHS.md)**.

## Documentation

- **Getting started:** [Guide](./docs/GUIDE.md) · [CLI reference](./docs/CLI.md) · [Configuration paths](./docs/CONFIG_PATHS.md) · [ACP editor integration](./docs/ACP.md)
- **Features & troubleshooting:** [Reasoning language](./docs/REASONING_LANGUAGE.md) · [Subagent profiles](./docs/SUBAGENT_PROFILES.md) · [Context Engine v2](./docs/SESSION_MEMORY_RETRIEVAL.md) · [Capability diagnostics](./docs/CAPABILITY_DIAGNOSTICS.md) · [Recovery and updates](./docs/RECOVERY.md) · [Checkpoints & rewind](./docs/CHECKPOINTS.md)
- **Engineering & migration:** [Spec](./docs/SPEC.md) · [Task contracts & pause policy](./docs/TASK_CONTRACT.md) · [Tool contract](./docs/TOOL_CONTRACT.md) · [Migrating from 0.x](./docs/MIGRATING.md)
- **Extension development:** [Extensions](./docs/EXTENSIONS.md) · [Plugin packages and Manifest v1](./docs/PLUGIN_PACKAGES.md) · [Extension Protocol](./docs/EXTENSION_PROTOCOL.md) · [Go SDK and starter](./sdk/go/README.md)

## Star History

<a href="https://www.star-history.com/?repos=pattycorp%2FDeepSeek-PattyCode&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-dark.svg" />
   <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-light.svg" />
   <img alt="Star History Chart" src="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-light.svg" />
 </picture>
</a>

<br/>

## Acknowledgments

A small list of folks whose work has shaped Patty Code the most — the current top 20 contributors by commit count. The full contributor graph is on [GitHub](https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors?all=1).

<!-- patty-code-top-contributors:start -->
| Contributor | Contributor | Contributor | Contributor |
| --- | --- | --- | --- |
| [**SivanCola**](https://github.com/SivanCola) | [**pattycorp**](https://github.com/pattycorp) | [**ttmouse**](https://github.com/ttmouse) | [**lifu963**](https://github.com/lifu963) |
| **patty** | [**HUQIANTAO**](https://github.com/HUQIANTAO) | [**GTC2080**](https://github.com/GTC2080) | [**light-front-theory**](https://github.com/light-front-theory) |
| **merge-order-check** | [**Li-Charles-One**](https://github.com/Li-Charles-One) | [**eghrhegpe**](https://github.com/eghrhegpe) | **wufengfan** |
| [**CVEngineer66**](https://github.com/CVEngineer66) | [**dependabot[bot]**](https://github.com/apps/dependabot) | [**lanshi17**](https://github.com/lanshi17) | [**SuMuxi66**](https://github.com/SuMuxi66) |
| [**CnsMaple**](https://github.com/CnsMaple) | [**cyq1017**](https://github.com/cyq1017) | [**JesonChou**](https://github.com/JesonChou) | [**XTLine**](https://github.com/XTLine) |
<!-- patty-code-top-contributors:end -->

Special thanks to [**Bernardxu123**](https://github.com/Bernardxu123) for designing the project logo and intro video.

<p align="center">
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=pattycorp/DeepSeek-PattyCode&max=100&columns=12" alt="Contributors to Patty Code" width="860"/>
  </a>
</p>

<br/>

---

<p align="center">
  <sub>MIT — see <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub>Built by <strong>Patty Co., Ltd.</strong> (<strong>주식회사 패티</strong>) with the <a href="https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors">Patty Code community</a></sub>
</p>

---

<p align="center"><sub><strong>Support this project</strong></sub></p>

If Patty Code has been useful and you'd like to say thanks, you can. It stays a coffee, not a contract — donations don't buy feature priority or change how issues get triaged.

- **PayPal** — [paypal.me/yuhuahui](https://paypal.me/yuhuahui)
