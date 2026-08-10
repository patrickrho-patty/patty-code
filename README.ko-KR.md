<p align="center">
  <img src="docs/logo-ghost-wave-effect.svg" alt="Patty Code" width="360"/>
</p>

<p align="center">
  <a href="./README.md">English</a>
  &nbsp;·&nbsp;
  <strong>한국어</strong>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.ko-KR.md">가이드</a>
  &nbsp;·&nbsp;
  <a href="./docs/ACP.ko-KR.md">ACP</a>
  &nbsp;·&nbsp;
  <a href="./docs/EXTENSIONS.ko-KR.md">확장</a>
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

<p align="center"><strong>오픈소스 · MIT · 한국 1등 · 한국 최초의 상용 한국어 특화 코딩 에이전트 하네스</strong></p>
<h3 align="center">Patty Code는 한국 개발자를 위해 먼저 설계되었습니다.</h3>
<p align="center">Patty Code는 <strong>주식회사 패티</strong>(<strong>Patty Co., Ltd.</strong>)가 만드는 한국어 워크플로 중심의 코딩 에이전트 하네스입니다. 하나의 로컬 엔진 위에 터미널, 데스크톱, 브라우저, ACP 에디터 진입점을 올리고, 한국어 reasoning 지원, HWPX 파싱, IME 조합 보존, 커서/캐럿 복원, 초성 슬래시 명령, 장시간 자율 실행 제어까지 한국 개발자가 실제로 불편해하던 지점을 제품 수준으로 다듬었습니다.</p>

<div align="center">
  <video src="https://github.com/user-attachments/assets/ab2f3878-e224-4931-8254-060e7695cfb9" controls preload="metadata" width="560"></video>
</div>

<br/>

> [!IMPORTANT]
> **커뮤니티** — 설정 도움말, 한국어 워크플로 사례, 기능 아이디어를 나누는 Discord입니다. → **<https://discord.gg/XF78rEME2D>**

<br/>

## 왜 Patty Code인가

- **한국어 우선 설계.** Patty Code는 한국어를 번역 레이어로 덧씌운 제품이 아니라, `ko-KR`를 핵심 모드로 다루는 한국어 퍼스트 하네스입니다.
- **한국어 입력이 끊기지 않습니다.** 데스크톱 컴포저는 IME 조합 상태를 보존하고, Enter가 조합 확정을 깨지 않으며, 조합 종료 뒤 커서 위치도 안정적으로 복원합니다.
- **초성 명령 UX.** 내장 슬래시 명령은 한국어 이름, 초성 별칭, 영어 이름으로 모두 접근할 수 있고, 애매한 초성 입력은 자동 실행하지 않도록 설계되어 있습니다.
- **한국 문서 워크플로.** Patty Code는 HWPX 파싱을 지원하며, 한국 팀의 문서 중심 개발 흐름까지 염두에 두고 설계되었습니다.
- **혼합 언어에 강합니다.** 한국어, 영어, 기타 CJK 문자가 섞인 입력에서도 폭 계산, 입력 처리, 명령 상호작용이 자연스럽게 동작합니다.
- **자율 실행을 통제할 수 있습니다.** Plan 모드, 권한 승인, 워크스페이스 샌드박스, 체크포인트, 브랜치, 되감기 덕분에 긴 자율 실행도 읽고 멈추고 되돌릴 수 있습니다.
- **상용 하네스 수준의 완성도.** 오픈소스 MIT 프로젝트이면서도 모델, 도구, 정책, 복구, UX를 실제 운영 관점에서 묶어낸 하네스입니다.
- **다중 모델/다중 제공자.** DeepSeek는 하나의 사전 설정일 뿐이며, OpenAI 호환 엔드포인트와 실행기/플래너 조합도 1급 기능으로 다룹니다.

## 핵심 기능

- **구성 중심.** Provider, 에이전트, 활성 도구, 플러그인을 모두 `patty.toml`에서 선언합니다.
- **Desktop + CLI + Browser + ACP.** 하나의 로컬 Patty Code 엔진을 네 가지 방식으로 사용할 수 있습니다.
- **플러그인 확장.** MCP 서버와 Extension Protocol 사이드카가 도구, 프롬프트, Provider, 리소스, 구조화된 UI를 추가할 수 있습니다.
- **캐시 친화적인 컨텍스트 유지.** 시작 시 안정적인 환경 요약을 주입하고, 오래된 도구 출력을 압축 전에 정리하며, 도구 스키마 계약을 회귀 검토용으로 문서화합니다.
- **배포가 단순합니다.** `CGO_ENABLED=0` 단일 바이너리와 6개 타깃 교차 빌드를 제공합니다.

## 설치

CLI/TUI, 데스크톱 앱, VS Code 확장은 모두 같은 로컬 Patty Code 엔진을 사용합니다.

### 경로 A: CLI / TUI

지원 플랫폼에서는 npm으로, macOS에서는 Homebrew로 설치할 수 있습니다.

```sh
npm i -g patty code                  # 모든 OS; 사전 빌드된 네이티브 바이너리 설치
brew install pattycorp/patty/patty code   # macOS
```

사전 빌드 아카이브(`darwin|linux|windows × amd64|arm64`)와 `SHA256SUMS`는 모든 [GitHub 릴리스](https://github.com/pattycorp/DeepSeek-PattyCode/releases)에 포함됩니다.

### 경로 B: 데스크톱 앱

최신 데스크톱 빌드는 [공식 다운로드 페이지](https://patty-code.io/?download=desktop#start)에서 받을 수 있습니다.

| 플랫폼 | 패키지 | 아키텍처 |
| --- | --- | --- |
| macOS | Universal `.dmg` 또는 `.zip` | Apple Silicon / Intel |
| Windows | 설치형 `.exe` 또는 휴대형 `.zip` | x64 / ARM64 |
| Linux | `.deb` 또는 `.tar.gz` | x64 |

Windows 설치 프로그램은 [SignPath.io](https://signpath.io/)를 통해 코드 서명됩니다.

### 경로 C: VS Code 확장

먼저 경로 A를 완료하세요. 확장은 CLI를 번들하지 않고, 로컬 `patty code acp` 백엔드를 시작해 네이티브 채팅, 에디터 컨텍스트, 도구 승인, 모델 선택, 워크스페이스 세션을 제공합니다.

- **VS Code:** [Visual Studio Marketplace에서 설치](https://marketplace.visualstudio.com/items?itemName=SivanLiu.patty-agent)
- **VSCodium / Eclipse Theia:** [Open VSX Registry에서 설치](https://open-vsx.org/extension/SivanLiu/patty-code-agent)
- **확장 ID:** `SivanLiu.patty-agent` · [소스 및 사용 가이드](https://github.com/SivanCola/patty-code-vscode)

### 경로 D: 소스에서 빌드

```sh
git clone https://github.com/pattycorp/DeepSeek-PattyCode.git
cd DeepSeek-PattyCode
make build      # -> bin/patty(.exe)
make cross      # -> dist/ (darwin|linux|windows × amd64|arm64)
```

## 빠른 시작

### CLI / TUI

```sh
patty code setup
patty code
patty code run "implement the TODOs in main.go"
```

대화형 세션에서는 프로젝트 지침을 만들고 싶을 때 `/init`을 실행하세요.

### 데스크톱 앱

[공식 다운로드 페이지](https://patty-code.io/?download=desktop#start)에서 설치 파일을 내려받아 실행한 뒤, 앱 안에서 Provider와 모델을 설정하면 됩니다. 데스크톱 앱만 사용할 때는 위 CLI 명령이 필수는 아닙니다.

고급 CLI 사용법과 구성은 **[CLI 문서](./docs/CLI.ko-KR.md)**, **[가이드](./docs/GUIDE.ko-KR.md)**, **[구성 경로](./docs/CONFIG_PATHS.md)** 를 참고하세요.

## 문서

- **시작하기:** [가이드](./docs/GUIDE.ko-KR.md) · [CLI 문서](./docs/CLI.ko-KR.md) · [ACP 에디터 연동](./docs/ACP.ko-KR.md)
- **기능 및 문제 해결:** [Reasoning language](./docs/REASONING_LANGUAGE.ko-KR.md) · [기능 진단](./docs/CAPABILITY_DIAGNOSTICS.ko-KR.md) · [체크포인트와 되감기](./docs/CHECKPOINTS.md)
- **엔지니어링 문서:** [Spec](./docs/SPEC.md) · [Tool contract](./docs/TOOL_CONTRACT.md) · [0.x에서 마이그레이션](./docs/MIGRATING.md)
- **확장 개발:** [Extensions](./docs/EXTENSIONS.ko-KR.md) · [Plugin packages](./docs/PLUGIN_PACKAGES.ko-KR.md) · [Extension Protocol](./docs/EXTENSION_PROTOCOL.ko-KR.md) · [Go SDK](./sdk/go/README.md)

## Star History

<a href="https://www.star-history.com/?repos=pattycorp%2FDeepSeek-PattyCode&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-dark.svg" />
   <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-light.svg" />
   <img alt="Star History Chart" src="https://raw.githubusercontent.com/pattycorp/DeepSeek-PattyCode/star-history/assets/star-history/star-history-light.svg" />
 </picture>
</a>

<br/>

## 감사의 말

Patty Code에 큰 영향을 준 기여자들을 소개합니다. 전체 기여자 그래프는 [GitHub](https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors?all=1)에서 볼 수 있습니다.

<!-- patty-code-top-contributors:start -->
| Contributor | Contributor | Contributor | Contributor |
| --- | --- | --- | --- |
| [**SivanCola**](https://github.com/SivanCola) | [**pattycorp**](https://github.com/pattycorp) | [**ttmouse**](https://github.com/ttmouse) | [**lifu963**](https://github.com/lifu963) |
| **patty** | [**HUQIANTAO**](https://github.com/HUQIANTAO) | [**GTC2080**](https://github.com/GTC2080) | [**light-front-theory**](https://github.com/light-front-theory) |
| **merge-order-check** | [**Li-Charles-One**](https://github.com/Li-Charles-One) | [**eghrhegpe**](https://github.com/eghrhegpe) | **wufengfan** |
| [**CVEngineer66**](https://github.com/CVEngineer66) | [**dependabot[bot]**](https://github.com/apps/dependabot) | [**lanshi17**](https://github.com/lanshi17) | [**SuMuxi66**](https://github.com/SuMuxi66) |
| [**CnsMaple**](https://github.com/CnsMaple) | [**cyq1017**](https://github.com/cyq1017) | [**JesonChou**](https://github.com/JesonChou) | [**XTLine**](https://github.com/XTLine) |
<!-- patty-code-top-contributors:end -->

특별히 프로젝트 로고와 인트로 영상을 디자인한 [**Bernardxu123**](https://github.com/Bernardxu123)에게 감사드립니다.

<p align="center">
  <a href="https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=pattycorp/DeepSeek-PattyCode&max=100&columns=12" alt="Contributors to Patty Code" width="860"/>
  </a>
</p>

<br/>

---

<p align="center">
  <sub>MIT — <a href="./LICENSE">LICENSE</a> 참고</sub>
  <br/>
  <sub><strong>주식회사 패티</strong>(<strong>Patty Co., Ltd.</strong>)와 <a href="https://github.com/pattycorp/DeepSeek-PattyCode/graphs/contributors">Patty Code 커뮤니티</a>가 함께 만들고 있습니다.</sub>
</p>

---

<p align="center"><sub><strong>프로젝트 후원</strong></sub></p>

Patty Code가 도움이 되었고 응원하고 싶다면 후원할 수 있습니다. 다만 후원은 기능 우선순위나 이슈 처리 순서를 바꾸지 않습니다.

- **PayPal** — [paypal.me/yuhuahui](https://paypal.me/yuhuahui)
