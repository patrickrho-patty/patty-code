# Patty Code 플러그인 패키지

Patty Code 플러그인 패키지는 skills, hooks, MCP 서버, prompts, themes, code extensions를 하나의 설치 가능한 단위로 묶습니다.

## CLI 모드

터미널에서 플러그인 패키지를 설치하거나 관리할 때는 `patcode plugin`을 사용하세요. 플러그인 패키지는 Patty Code 홈 디렉터리 아래에 전역으로 설치됩니다.

### CLI에서 설치

`install`은 소스 하나를 받습니다:

- `git:github.com/obra/superpowers` 또는 `https://github.com/obra/superpowers` 같은 GitHub 저장소.
- `https://github.com/owner/repo/tree/main/path/to/plugin` 같은 GitHub 브랜치 또는 하위 디렉터리 URL.
- `patty-plugin.json`, `.codex-plugin/plugin.json` 또는 `.claude-plugin/plugin.json`을 포함하는 로컬 디렉터리.

파일을 쓰지 않고 설치 계획을 미리 보려면:

```bash
patcode plugin install git:github.com/obra/superpowers --dry-run
```

계획을 검토한 후 플러그인을 설치하려면:

```bash
patcode plugin install git:github.com/obra/superpowers --yes
```

명시적인 이름으로 설치하거나 같은 이름의 설치된 플러그인을 교체하려면:

```bash
patcode plugin install git:github.com/obra/superpowers --name superpowers --replace --yes
```

개발자 모드에서 로컬 디렉터리를 사용하려면:

```bash
patcode plugin install /path/to/plugin --link --replace --yes
```

CLI 설치 플래그:

- `--dry-run`은 파일을 쓰지 않고 설치를 계획하고 검증합니다.
- `--yes`는 파일을 쓰는 모든 설치에 필요합니다.
- `--replace`는 소스가 같은 이름의 설치된 플러그인을 교체할 수 있게 합니다.
- `--name <name>` 또는 `--name=<name>`은 이번 설치에서 플러그인 매니페스트의 이름을 덮어씁니다.
- `--link`는 로컬 플러그인 디렉터리를 Patty Code의 플러그인 저장소에 복사하는 대신 링크합니다. 해당 디렉터리를 이동하거나 삭제하면 링크된 플러그인이 깨집니다.

`--dry-run` 또는 `--yes` 없이 `patcode plugin install <source>`를 실행하면 파일 쓰기를 거부하고 두 플래그 중 하나로 다시 실행하라는 안내를 출력합니다. 설치 및 제거 명령은 데스크톱 UI가 사용하는 동일한 install-source 백엔드의 구조화된 JSON 응답을 출력합니다.

설치된 플러그인 상태는 다음 위치에 저장됩니다:

```text
~/.patty/plugin-packages.json
~/.patty/plugins/<name>/
```

### CLI에서 관리

설치된 플러그인 나열:

```bash
patcode plugin list
```

플러그인 하나의 메타데이터, 루트, 소스, 내보낸 capability 수 표시:

```bash
patcode plugin show superpowers
```

`show`는 가능한 경우 구체적인 capability 인벤토리도 출력합니다:

- **skills**: 제안된 `/<plugin>:<skill>` 호출과 설명을 포함합니다.
- **commands**: `/<plugin>:<command>` 호출, 인수 힌트, 설명을 포함합니다.
- **hooks**: 수명 주기 이벤트, matcher, 명령 또는 컨텍스트 파일을 나열합니다.
- **mcpServers**: 서버 이름, 전송 방식, 실행 대상을 나열합니다.

매니페스트와 skill 루트를 읽을 수 있는지 확인:

```bash
patcode plugin doctor superpowers
```

워크스페이스 전체의 capability 보고서(skills, hooks, MCP 병합, 패키지 루트)는 [Capability diagnostics](../reference/CAPABILITY_DIAGNOSTICS.md)를 참조하세요:

```bash
patcode doctor capabilities --json
# 데스크톱: Settings → Diagnostics
# 에이전트: /patty-guide
```

플러그인을 제거하지 않고 활성화 또는 비활성화:

```bash
patcode plugin disable superpowers
patcode plugin enable superpowers
```

플러그인 제거:

```bash
patcode plugin remove superpowers --yes
```

`remove`는 `uninstall`을 별칭으로도 받습니다. 상태를 쓰고 복사된 플러그인 콘텐츠를 제거하므로 `--yes`가 필요합니다. 링크된 로컬 플러그인의 경우 외부 소스 디렉터리는 그대로 남습니다.

### CLI에서 설치된 플러그인 사용

설치된 플러그인은 별도의 채팅 화면을 열지 않습니다. 플러그인이 활성화되면 Patty Code는 해당 capability를 일반 대화형 세션에 로드합니다:

- 대화형 세션에서 `/plugins`를 실행하면 설치된 플러그인 패키지 목록을 볼 수 있습니다. `/plugins show <name>`을 실행하면 채팅을 벗어나지 않고 플러그인의 내보낸 skills, hooks, MCP 서버, 사용 힌트를 확인할 수 있습니다.
- **Skills**는 `/skills`에 표시됩니다. `/<plugin>:<skill> [args]`로 플러그인 skill을 호출하거나, 자연스럽게 요청해서 에이전트가 설명에 맞는 skill을 선택하게 할 수 있습니다.
- **Hooks**는 `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse` 같은 구성된 수명 주기 이벤트에서 자동으로 실행됩니다.
- **MCP 서버**는 일반 MCP/도구 흐름에 참여합니다. 원하는 작업을 요청하면 Patty Code가 관련이 있을 때 플러그인의 도구를 호출할 수 있습니다.

세션이 이미 실행 중인 상태에서 별도의 터미널로 플러그인을 설치, 활성화, 비활성화 또는 업데이트한 경우, 새 `patty` 세션을 시작하거나 `/skills`를 다시 열어 현재 세션이 예상한 skills를 인식하는지 확인하세요.

## 데스크톱 설정

CLI를 사용하지 않고 플러그인 패키지를 설치하고 관리하려면 **Settings -> Plugins**를 여세요.

### 플러그인 설치

설치 프로그램에는 두 가지 모드가 있습니다:

- **로컬 폴더**: **Choose plugin folder**를 클릭하고 디스크에서 플러그인 디렉터리를 선택합니다. 선택한 경로는 버튼 옆에 표시됩니다.
- **Git 저장소**: `git:github.com/obra/superpowers` 같은 Git 소스를 입력합니다. **Install name (optional)**은 이번 설치의 플러그인 매니페스트 이름을 덮어쓰거나 교체할 수 있습니다.

소스와 옵션을 선택한 후 작업 버튼을 사용하세요:

- **Preview**는 파일을 쓰지 않고 소스를 검증하고 계획된 설치 작업을 보여줍니다.
- **Install plugin**은 현재 옵션으로 선택한 소스를 설치합니다.
- **Refresh plugins**는 디스크와 구성에서 설치된 플러그인 목록을 다시 로드합니다.

설치 프로그램 옵션:

- **Overwrite same-name plugin**은 현재 소스가 같은 이름의 설치된 플러그인을 교체할 수 있게 합니다. 중복 이름 설치 시 기존 콘텐츠를 교체하는 대신 실패해야 한다면 이 옵션을 끄세요.
- **Developer mode: link source folder**는 **로컬 폴더** 설치에서 나타납니다. 선택한 디렉터리를 Patty Code의 플러그인 저장소에 복사하는 대신 링크합니다. 플러그인을 개발하거나 디버깅할 때 사용하세요. 선택한 디렉터리를 이동하거나 삭제하면 링크된 플러그인이 깨집니다.

Preview는 새 Git 소스 또는 로컬 플러그인 디렉터리에 가장 안전한 첫 단계입니다.

### 설치된 플러그인 관리

설치된 플러그인 목록에는 각 플러그인 패키지와 내보낸 skills, hooks, MCP 서버가 표시됩니다. 앱 외부에서 플러그인 파일을 편집하거나 구성을 변경한 후에는 **Refresh plugins**를 사용하세요.

플러그인 행을 펼쳐 관리하세요:

- 플러그인 활성화 또는 비활성화.
- 플러그인의 내보낸 skills, hooks, MCP 서버에 대한 **How to use** 읽기.
- **Update**는 업데이트 소스를 사용할 수 있을 때 설치된 플러그인을 가져오거나 새로 고칩니다.
- **Doctor**는 플러그인 매니페스트를 확인하고 경고 또는 진단 정보를 보고합니다.
- **Remove plugin**은 확인 후 패키지를 제거합니다.

### 데스크톱에서 설치된 플러그인 사용

데스크톱 설정 페이지는 CLI와 동일한 런타임 모델을 사용합니다:

- 설치된 플러그인을 펼치면 **How to use** 섹션을 볼 수 있습니다.
- 데스크톱 세션에서 `/plugins`를 입력하면 설치된 플러그인 목록을, `/plugins show <name>`을 입력하면 채팅 화면에서 동일한 사용 세부 정보를 볼 수 있습니다.
- Skills는 `/superpowers:writing-plans` 같은 패키지 한정 직접 명령으로 표시되며, 세션의 `/skills`에서도 찾을 수 있습니다.
- 플러그인 명령은 `/superpowers:plan` 같은 패키지 한정 이름으로 표시되고 호출됩니다.
- Hooks와 MCP 서버는 투명성을 위해 나열됩니다. 수동 "run" 버튼이 필요하지 않습니다: 활성화된 hooks는 자동으로 트리거되고, MCP 도구는 일반적인 도구 사용으로 이용할 수 있습니다.
- 현재 열려 있는 세션이 플러그인 변경 사항을 반영하지 않으면 플러그인 목록을 새로 고치고 새 세션을 여세요.

## 네이티브 매니페스트

Patty Code 플러그인은 플러그인 루트에 `patty-plugin.json`을 선언할 수 있습니다:

```json
{
  "name": "example",
  "version": "1.0.0",
  "description": "Example plugin",
  "skills": "skills",
  "hooks": {
    "SessionStart": [
      {
        "command": "hooks/session-start",
        "args": [],
        "description": "Load startup context"
      },
      {
        "command": "printf 'ready' && ./hooks/audit",
        "shell": "bash",
        "description": "Run a compound shell script"
      }
    ]
  },
  "mcpServers": {
    "helper": {
      "command": "bin/helper"
    }
  }
}
```

상대 경로는 플러그인 루트 내부에서 해석됩니다. Patty Code는 플러그인 설치 중에 타사 설치 스크립트를 실행하지 않습니다.

플러그인 hook 실행은 명시적입니다:

- `args`가 있으면(`"args": []` 포함) hook은 **exec form**을 사용합니다. `command`는 실행 파일이고 모든 인수는 셸 파싱이나 보간 없이 그대로 전달됩니다.
- `args`가 없고 `shell`이 있으면 hook은 **shell form**을 사용합니다. 전체 `command`는 변경 없이 `bash`, `powershell`/`pwsh`, `cmd`(Windows 전용) 또는 `auto`에 전달됩니다. Windows에서 `auto`는 Git Bash를 선호하고 PowerShell로 대체됩니다.
- 두 필드 모두 선언하지 않은 기존 네이티브 hooks는 역사적인 Patty Code 셸 명령 동작을 유지합니다. `shellCommand: true`는 shell form의 레거시 표기로 계속 지원됩니다.

## 매니페스트 v2(확장)

네이티브 Patty Code 확장은 정확한 v2 `apiVersion`을 사용합니다:

```json
{
  "apiVersion": "patty-code.io/plugin/v2",
  "name": "example",
  "version": "1.0.0",
  "description": "Example extension",
  "requires": [],
  "provides": [
    {
      "namespace": "plugin/example",
      "kind": "interceptors",
      "id": "default",
      "version": "1.0.0"
    }
  ],
  "contributes": {
    "skills": ["skills"],
    "agents": ["agents"],
    "commands": ["commands"],
    "prompts": ["prompts"],
    "hooks": {},
    "mcpServers": {},
    "themes": ["themes/*.patty-theme"]
  },
  "runtime": {
    "command": "${PATTY_CODE_PLUGIN_ROOT}/bin/example",
    "args": [],
    "env": {},
    "required": true,
    "priority": 0,
    "intercepts": ["input.receive", "tool.before"],
    "replaces": [],
    "capabilities": ["interceptors"]
  }
}
```

파싱 규칙:

- 네이티브 `patty-plugin.json` 매니페스트는 정확한 `patty-code.io/plugin/v2` 값을 선언해야 합니다. v1과 버전 누락은 거부되며, v1 이중 읽기나 자동 마이그레이션 경로는 없습니다.
- v2는 엄격합니다: 루트 또는 `contributes`/`runtime` 아래에 중첩된 알 수 없는 필드는 필드 경로를 명명하는 오류가 되므로, 오타가 capability를 조용히 비활성화하는 대신 명확하게 실패합니다.
- 마이너 별칭(`patty-code.io/plugin/v2.0`, `v2.1`, …)과 알 수 없는 메이저 버전은 거부됩니다.
- `requires`와 `provides`는 Sidecar 핸드셰이크에 대해 적용되는 종속성 제약과 capability 상한을 선언합니다.
- v2는 지원되는 최상위 리소스 필드(`skills`, `hooks`, `mcpServers`, …)를 `contributes`와 혼합할 수 있습니다: 동일한 경로는 중복 제거되고, 서로 다른 두 정의를 가진 같은 키는 키를 명명하는 매니페스트 오류가 됩니다.
- 모든 상대 경로와 glob은 플러그인 루트 안에 있어야 합니다: 트래버설, 절대 경로, 심볼릭 링크 탈출, 일반 파일이 아닌 테마 파일은 거부됩니다.

새 리소스 유형:

- `prompts`는 commands와 동일한 의미론과 인수 치환을 사용하는 프롬프트 템플릿이며 `/<plugin>:<name>`으로 호출됩니다. `commands`는 호환 가능한 별칭으로 유지됩니다.
- `themes`는 데스크톱 설정에서 플러그인 테마(ID `plugin:<plugin>:<theme>`)로 읽기 전용으로 표시되는 `.patty-theme` 파일입니다. 사용자 테마 라이브러리에는 복사되지 않습니다. 테마가 활성 상태인 동안 플러그인이 비활성화되거나 제거되면 데스크톱은 기본 스타일로 돌아가지만 ID는 유지되므로, 같은 플러그인을 다시 설치하면 테마가 복원됩니다.

`runtime` 블록은 code extension — Patty Code가 실행하고 Extension Protocol(stdio 위의 JSON-RPC 2.0; 메서드 인덱스는 `docs/extensions/EXTENSION_PROTOCOL.generated.md`, Go SDK는 `sdk/go/README.md` 참조)로 통신하는 사이드카 프로세스 — 을 선언합니다:

- `command`/`args`/`env`는 **exec form 전용**입니다 — 명령은 실행 파일이며(셸로 해석되지 않음), `${PATTY_CODE_PLUGIN_ROOT}`는 설치된 플러그인 루트로 확장됩니다.
- `intercepts`는 확장이 가로채려는 이벤트를 나열합니다(예: `input.receive`, `tool.before`, `permission.decision`); `replaces`는 확장이 소유할 수 있는 교체 슬롯을 선언합니다(`system_prompt`, `context`, `provider_request`, `provider_response`, `compaction`, `session_policy`, `permission`, `frontend_events`, `tool:<name>`, `provider:<ref>`). 각 슬롯은 설치된 모든 플러그인에서 정확히 하나의 소유자를 갖습니다. 충돌이 발생하면 두 소스를 모두 명명한 채 빌드가 실패합니다.
- `capabilities`는 전체 기능 계열을 제어합니다: `interceptors`, `strategies`, `providers`, `ui`. 사이드카가 매니페스트를 벗어나 선언하는 모든 것은 핸드셰이크 중에 거부됩니다.
- 확장이 제공하는 provider 모델은 모델 선택기에 `plugin/<plugin>/<provider>/<model>`로 표시됩니다. 이 ref는 `default_model`(첫 부팅 포함)과 `/model`, 데스크톱, ACP 모델 전환에서도 작동합니다.

**전체 신뢰(Full trust).** code extension은 샌드박스 밖에서 필터링되지 않은 상속 환경으로 실행됩니다. 전체 세션과 환경을 읽고, 권한을 우회하며, 머신을 직접 조작할 수 있습니다. 확장의 `permission.decision` "allow"는 호스트의 deny를 덮어씁니다. `runtime` 블록이 있는 플러그인을 설치, 업데이트, 교체 또는 `--link`하는 것 자체가 승인입니다 — 두 번째 확인 프롬프트는 없으며, `--link`는 변경된 콘텐츠를 자동으로 계속 신뢰합니다. 따라서 설치 미리 보기, `patcode plugin show`, capability 진단, 데스크톱 설치 프로그램은 런타임 명령, interceptors, 교체 슬롯, provider/UI capability와 함께 눈에 띄는 `FULL TRUST` 블록을 표시합니다. 설치 전에 해당 블록을 검토하고, 완전히 신뢰하는 런타임만 설치하세요. 플러그인 흐름을 통해 설치된 플러그인만 런타임을 시작할 수 있습니다. 프로젝트 구성은 절대 런타임을 선언할 수 없습니다.

## Codex & Claude 호환성

Patty Code는 `.codex-plugin/plugin.json`의 Codex 플러그인 매니페스트와 `.claude-plugin/plugin.json`의 Claude 플러그인 매니페스트도 읽습니다. 설치 미리 보기는 `full`, `partial` 또는 `none` 호환성을 보고하고, 매핑된 capability를 나열하며, 건너뛴 모든 항목을 식별합니다. 매핑된 capability가 없는 비네이티브 패키지는 사용할 수 없는 설치로 기록되는 대신 차단됩니다. `full`은 매니페스트의 모든 선언된 capability가 파싱되어 Patty Code 구성 요소에 매핑되었음을 의미합니다. 이것만으로 가져온 hook이 내릴 수 있는 모든 런타임 결정이 존중된다는 것을 보장하지는 않습니다. `PreToolUse`/`PermissionRequest` "deny"와 `PermissionRequest` "allow"는 구현되어 있지만, hook의 `updatedInput` 또는 `PreToolUse`의 `ask`/`defer` 결정은 매니페스트의 어떤 것도 아니라 호출 시점에 스크립트의 stdout으로 결정되므로 설치 중에 플래그를 지정할 수 없습니다. 구현된 내용은 아래 hook 항목을 참조하세요. `.claude-plugin/marketplace.json`이 있는 GitHub 호스팅 다중 플러그인 마켓플레이스는 플러그인 항목이 `./plugins/example` 또는 `plugins/example` 같은 상대 문자열 소스를 사용할 때 저장소 루트에서 설치할 수 있습니다. 미리 보기는 아무것도 쓰기 전에 플러그인별 작업 하나를 보여줍니다. 선택적 설치 이름을 마켓플레이스 플러그인 이름으로 설정하면 해당 항목만 선택합니다. 객체 소스는 전체 커밋 SHA에 고정된 GitHub 저장소 URL에 대해서만 허용됩니다. 고정되지 않은 외부 문자열, npm, `strict: false` 및 기타 고급 마켓플레이스 프로토콜은 일괄 설치에서 건너뛰고 이름으로 선택하면 거부됩니다. Superpowers 및 Claude 스타일 skill 팩 같은 패키지의 경우 Patty Code는 다음을 매핑합니다:

- `skills`를 Patty Code skill 루트에 매핑합니다. `skills` 필드를 선언하지 않은 Claude 매니페스트는 Claude 자체의 자동 검색과 일치하게 기존의 `skills/`(또는 `.claude/skills/`) 디렉터리로 대체됩니다. 플러그인 skills는 `/<plugin>:<skill>`로 정식 표시되고 호출됩니다. 모호하지 않은 `/<skill>`은 숨겨진 호환 별칭으로 계속 허용됩니다. 프로젝트 및 사용자 skills는 짧은 이름을 유지하는 반면, 여러 플러그인의 동일 이름 skills는 정규화된 이름으로만 독립적으로 주소 지정할 수 있습니다. 이 사용자 대상 네임스페이스는 모델 skill 인덱스나 `run_skill` 도구의 기본 skill 식별자를 변경하지 않습니다.
- `commands/`(및 `.claude/commands/`)를 Patty Code 사용자 지정 슬래시 명령에 매핑합니다: 각 `<name>.md` 프롬프트 템플릿은 `/<plugin>:<name>`으로 정식 표시되고 호출되며, frontmatter `description` / `argument-hint` 및 `$ARGUMENTS` / `$1..$N` 치환이 적용됩니다. 모호하지 않은 `/<name>`은 숨겨진 호환 별칭으로 계속 허용되지만, 자동 완성, 도움말, 데스크톱 메뉴, ACP 명령 검색 및 모델에 보이는 명령 목록에서는 제외됩니다. 사용자 및 프로젝트가 작성한 명령은 짧은 이름을 소유하며, 여러 플러그인이 같은 명령 이름을 내보낼 때 짧은 별칭은 만들어지지 않습니다. 명시적 사용자 지정 명령이 정규화된 이름을 차지할 수도 있으며, 데스크톱 플러그인 세부 정보가 해당 충돌을 보고합니다. 네이티브 `patty-plugin.json` 매니페스트는 `"commands"` 경로 목록으로 동일한 것을 명시적으로 선언할 수 있습니다.
- `agents/*.md`를 수동으로 호출되는 플러그인 소유 subagent 프로필에 매핑합니다. Claude 모델 별칭은 활성 Patty Code 모델을 상속하며, 인라인 `tools` 목록은 `mcp__*__search` 같은 와일드카드 MCP 이름을 포함해 Patty Code 도구 이름에 매핑됩니다. 에이전트는 `/<plugin>:agent:<name>`을 사용하므로 상위 에이전트와 skill이 서로를 가리지 않고 같은 이름을 공유할 수 있습니다.
- `hooks/session-start-codex`를 존재할 때 Patty Code `SessionStart` hook에 매핑합니다.
- 플러그인 루트의 `CLAUDE.md` 파일을 내장 `SessionStart` 컨텍스트 hook에 매핑합니다. 파일은 셸 명령을 실행하지 않고 Patty Code가 직접 읽습니다.
- 이벤트 이름이 일치하면 `.claude/settings.json` 및 `hooks/hooks.json` 명령 hooks를 Patty Code hook 이벤트에 매핑합니다. `matcher`, `args`, `shell`, `async`, `env` 및 timeout이 보존됩니다. Claude의 실행 계약이 유지됩니다: `args` 필드(빈 배열 포함)는 exec form을 선택하고 모든 인수를 그대로 보존하며, `args`를 생략하면 shell form을 선택하고 원시 명령을 선언된 Bash 또는 PowerShell 인터프리터에 전달합니다. `matcher`와 hook 스크립트가 보는 `tool_name`은 Patty Code 고유 도구 이름과 Claude 도구 이름 사이에서 변환되므로(`bash` ↔ `Bash`, `write_file` ↔ `Write`, ...) `"Bash"` 같은 matcher가 올바르게 발동합니다. Patty Code의 모든 subagent 생성 도구(`task`, `read_only_task`, `parallel_tasks`, 전용 `explore`/`research`/`review`/`security_review` 래퍼)는 Claude의 단일 `Agent` 도구에 매핑되며, matcher는 레거시 `Task` 이름을 계속 사용할 수 있습니다. 매핑된 모든 `Agent` 페이로드는 Claude가 요구하는 `prompt`와 `description`을 포함합니다. Patty Code는 자체 도구 호출에서 선택적 description을 생략한 경우 안정적인 작업 레이블을 제공합니다. Patty Code가 Claude와 다르게 이름을 지정하는 `tool_input` 키도 이름이 변경됩니다 — `path`는 `Read`/`Write`/`Edit`/`MultiEdit`의 경우 `file_path`로, `NotebookEdit`의 경우 `notebook_path`로, `Skill`의 경우 `name`/`arguments`가 `skill`/`args`로, 현재 `TaskOutput`/`TaskStop`의 경우 `job_id`가 `task_id`로, 전용 subagent 래퍼의 `task`는 `Agent`의 `prompt`가 되며, `parallel_tasks`는 하위 작업 프롬프트에서 `Agent`의 `prompt`를 합성합니다(`tasks`는 옆에 유지) — 따라서 `.tool_input.file_path` 또는 `.tool_input.prompt`를 읽는 가드가 빈 값으로 열리지 않고 대상을 볼 수 있습니다. 레거시 `BashOutput`/`KillShell` matcher는 발행된 이름과 필드가 현재 Claude 용어를 사용하는 동안 계속 발동합니다. `bash_output`은 `TaskOutput`의 필수 비차단 필드를 제공하며, `wait`도 `TaskOutput`에 매핑됩니다 — 정확히 하나의 작업을 기다릴 때 `task_id`를 포함하고, 무제한 대기의 경우 `0`ms 예산을 주장하는 대신 `TaskOutput`의 선택적 `timeout`을 생략합니다. `AskUserQuestion`은 생략된 `multiSelect:false`와 빈 옵션 설명을 제공하고, `TodoWrite`는 작업 콘텐츠에서 생략된 `activeForm`을 파생합니다. `NotebookEdit`은 또한 Patty Code가 수락한 별칭에서 `new_source`를 제공하거나 삭제/빈 셀 작업의 경우 빈 문자열을 제공합니다. 상대 `file_path`/`notebook_path` 값은 페이로드 `cwd`를 기준으로 절대 경로로 해석되어 Claude의 파일 도구 계약과 일치하므로, 접두사 일치 가드는 도구가 실제로 접근하는 경로를 검사합니다. `Bash`의 `tool_response`는 Claude의 `{stdout, stderr, interrupted}` 형태로 전달됩니다(Patty Code는 두 스트림을 `stdout`으로 결합하고, 실패 오류 텍스트는 `stderr`가 됩니다). 공식 보안 가이드 플러그인의 commit/push 검사가 이 형식을 읽습니다. 다른 도구의 응답은 원시 결과로 그대로 전달됩니다. 가져온 hooks는 `hook_event_name`을 포함해 Claude 호환 snake_case stdin 페이로드를 받습니다. 프로세스 실행 전에 호스트는 `${CLAUDE_PLUGIN_ROOT}`와 `${PATTY_CODE_PLUGIN_ROOT}`(중괄호 없는 `$NAME` 및 Windows `%NAME%` 표기 포함)를 확장하므로 플러그인 상대 경로가 대상 셸의 환경 변수 구문에 의존하지 않습니다. Windows에서 명시적 셸이 없는 shell-form hooks는 Patty Code의 셸 도구와 동일한 Git Bash 우선, PowerShell 대체 선택을 사용합니다. hook이 POSIX shebang 스크립트 파일을 가리키면 호스트는 Windows 경로를 Bash 호환 형식으로 변환합니다. 명시적 Bash hooks와 레거시의 순수 `sh -c`/`bash -c` hooks는 `cmd.exe`의 `PATH`에 없더라도 발견된 Git for Windows Bash를 통해 라우팅됩니다. 명시적 인터프리터 경로는 변경되지 않습니다. 사용 가능한 Bash가 설치되지 않은 경우 hook은 지역화된 `sh is not recognized` 출력 대신 명확한 필수 구성 요소 오류를 보고합니다. `[tools.shell] prefer = "bash"` 및 `path = ".../bash.exe"`로 구성된 비표준 또는 휴대용 Bash는 명시적 Bash hooks에서 재사용됩니다. `patcode plugin doctor <name>`과 `patcode doctor capabilities`는 첫 번째 hook 호출 전에 누락된 필수 셸을 보고합니다. 캡처된 레거시 코드 페이지 출력은 UI에 도달하기 전에 UTF-8로 정규화됩니다. `PreToolUse` 또는 `UserPromptSubmit` hook은 종료 코드 2 또는 종료 코드 0에서 JSON deny 형태(`PreToolUse`의 경우 `hookSpecificOutput.permissionDecision`, `UserPromptSubmit`의 경우 최상위 `decision:"block"`)로 여전히 거부할 수 있습니다. 가져온 `PermissionRequest` hook은 Claude 자체 계약에 맞춰 종료 코드 2 또는 `hookSpecificOutput.decision.behavior`를 통해 권한 대화 상자 자체에 응답합니다(알림만 하는 것이 아니라 deny 또는 auto-allow). `updatedInput`은 아직 도구 호출에 적용되지 않으며, hook의 `if` 조건 또는 `asyncRewake` 필드는 평가되지 않습니다. 패키지가 두 필드 중 하나, `Stop`/`SubagentStop` hook(Patty Code에서 턴을 차단할 수 없음) 또는 Patty Code가 무손실로 표현할 수 없는 세 가지 입력 중 하나를 다루는 matcher를 선언하면 구조화된 경고와 함께 부분 호환성이 보고됩니다: `WebFetch.prompt`, Patty Code `cell_number` 호출에 대한 `NotebookEdit.cell_id`, Patty Code `wait`가 여러/모든 작업을 다룰 때의 `TaskOutput.task_id`. 각 구조적 격차는 hooks 파일당 한 번 보고되므로 와일드카드 matcher 플러그인은 hook당 하나가 아닌 격차당 하나의 경고를 보게 됩니다.
- 플러그인 루트의 `.mcp.json`을 설치된 MCP 항목에 매핑합니다. Claude `local`은 stdio에 매핑되고, 비ASCII 표시 이름은 안정적인 내부 ID를 받으며, 중복 선언은 중복 제거됩니다. 가져온 서버는 기본적으로 `auto_start=false`입니다. 사용자가 필요할 때 연결하므로 시작 시 provider에 보이는 도구 스키마가 변경되지 않습니다.

지원되지 않는 Claude hook 항목 유형은 경고와 함께 건너뜁니다. Patty Code는 타사 설치 스크립트를 실행하지 않습니다.

플러그인 hooks는 다음 환경 변수를 받습니다:

- `PATTY_CODE_PLUGIN_ROOT`
- `PATTY_PLUGIN_NAME`
- `PATTY_PLUGIN_VERSION`
- `PATTY_HOME`
- `PATTY_WORKSPACE_ROOT`
- `CLAUDE_PROJECT_DIR`
- `CLAUDE_PLUGIN_ROOT`

## 데스크톱 백엔드 메서드

데스크톱은 Wails 메서드를 통해 플러그인 패키지 작업을 노출합니다:

- `Plugins`
- `PlanPluginInstall`
- `InstallPlugin`
- `RemovePlugin`
- `SetPluginEnabled`
- `UpdatePlugin`
- `PluginDoctor`
