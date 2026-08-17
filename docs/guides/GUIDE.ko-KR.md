# Patty Code 가이드

<a href="../../README.md">README</a>
&nbsp;·&nbsp;
<a href="./GUIDE.md">(English)</a>
&nbsp;·&nbsp;
<a href="../reference/SPEC.md">Spec</a>

> 일상적인 구성(configuration)과 사용 방법을 다룹니다. 엔지니어링 계약과 내부 구조(데이터 타입, 레지스트리, 패키지 레이아웃, 로드맵)는 **[Spec](../reference/SPEC.md)** 을 참조하세요.

## 목차

- [구성](#구성)
- [CLI 참조](./CLI.md)
- [환경 변수](#환경-변수)
- [Serve 웹 프론트엔드](#serve-웹-프론트엔드)
- [구성 경로](./CONFIG_PATHS.md)
- [추론 언어](../reference/REASONING_LANGUAGE.md)
- [작업 계약 및 일시 중지 정책](../reference/TASK_CONTRACT.md)
- [사용자 지정 OpenAI 호환 제공자](#사용자-지정-openai-호환-제공자)
- [데스크톱 훅](#데스크톱-훅)
- [키보드 단축키](#키보드-단축키)
- [권한 및 샌드박스](#권한-및-샌드박스)
- [기능 진단](#기능-진단)
- [플러그인(MCP)](#플러그인mcp)
- [슬래시 명령](#슬래시-명령)
- [내장 문서 검색](#내장-문서-검색)
- [@ 참조](#-참조)
- [이중 모델 협업](#이중-모델-협업)

## 구성

해석 순서는 **플래그(flag) > `./patty.toml` > 사용자 구성 파일 > 내장 기본값** 입니다. **Patty Code v1.8.1**부터 사용자 구성은 macOS/Linux에서 `~/.patty/config.toml`, Windows에서는 `%AppData%\patty\config.toml`에 위치합니다. 마이그레이션과 관련 데이터 경로는 [구성 경로](./CONFIG_PATHS.md)를 참조하세요. 사용자/글로벌 전용으로 표시된 필드는 `./patty.toml`로 재정의되지 않습니다. 제공자(provider) 항목은 비밀값을 `api_key_env`로 이름을 지정하며, 비밀값 자체는 CLI와 데스크톱이 공유하는 Patty Code 전역 `<Patty Code home>/.env`에 저장됩니다. 프로젝트 `.env`, 홈 `.env`, 상속된 셸 환경 변수, 레거시 자격 증명, OS 키링은 제공자 키의 런타임 폴백이 아닙니다. 레거시 자격 증명은 마이그레이션 소스로만 사용됩니다. 프로젝트 `.env`는 제공자 키나 Patty Code 제어 변수를 가져오지 않으면서 워크스페이스 범위의 비제공자 `${VAR}` 확장을 MCP/플러그인 설정에 계속 제공합니다. 전체 `config.toml` 및 `.env` 구조는 [구성 경로](./CONFIG_PATHS.md)를 참조하세요.

데스크톱과 CLI에서 표시되는 추론 언어를 설정하는 방법은 [추론 언어](../reference/REASONING_LANGUAGE.md)를 참조하세요.

```toml
default_model = "patty/medium"   # 실행 모델; 플래너를 추가하려면 [agent].planner_model 설정
# language    = "ko-KR"            # UI 언어; 비어 있으면 한국어 기본값, en이면 영어

[ui]
# shortcut_layout = "desktop"      # classic|desktop; 호환성 설정
# cursor_shape = "bar"             # block|underline|bar; CLI/TUI 텍스트 커서
show_turn_usage = false             # TUI에서 요청별 토큰/비용 내역 숨김; 기본값 true

[agent]
reasoning_language = "auto"      # 표시용 reasoning 텍스트: auto|ko-KR|en
# plan_mode_read_only_commands = ["gh issue view"]   # 레거시 호환 전용; Plan bash는 이제 Permissions 사용
# planner_model = "patty/medium"      # 선택적 저빈도 플래너
# subagent_model = "patty/medium"     # runAs=subagent 스킬용 선택적 기본값
# subagent_models = { review = "patty/medium", security_review = "patty/medium" }
# max_subagent_depth = 2              # 중첩 위임 깊이; 기존 단일 계층 경계는 1로 설정
# max_subagent_concurrency = 6        # 세션 전체 하위 에이전트 동시성(task/fleet/skills)
# max_parallel_writers = 3            # 겹치지 않는 write_paths를 가진 동시 작성자
tool_result_snip_ratio = 0.6       # 요약 압축 전 오래된 도구 출력 축약
compact_ratio = 0.9596935403266109 # 248124토큰 중 정확히 238123토큰에서 자동 압축
compact_force_ratio = 0.98         # 컨텍스트 한계 안전 경계

[[providers]]
name        = "patty"
kind        = "openai"
base_url    = "https://omni.agents.patty.io/v1"
model       = "medium"
api_key_env = "AGENTS_PATTY_API_KEY"
context_window = 248124

[tools]
enabled = []   # 생략/비움 = 모든 내장 도구
bash_timeout_seconds = 120   # 포그라운드 안전 상한; 도구별 상한 없음은 0으로 설정
mcp_startup_timeout_seconds = 30   # 백그라운드 initialize + tools/list 안전 상한
mcp_call_timeout_seconds = 300   # 기본 MCP 호출 안전 상한; 플러그인/도구별 재정의로 상향 가능

[environment]
enabled = true   # OS, 셸, 공통 도구의 안정적인 시작 요약 주입
offline = false  # 외부 네트워크 접근이 불가능할 때 true 설정; 헛된 재시도 방지
# [environment.tools]
# go = "/opt/homebrew/bin/go"   # 선택적 명시적 신뢰 경로; 워크스페이스 로컬 경로는 자동 실행되지 않음

[skills]
# paths = ["~/my-skills", "../shared/skills"]   # 추가 사용자 지정 스킬 루트
# excluded_paths = ["~/.agents/skills"]         # 폴더 삭제 없이 관례 루트 숨기기
# disabled_skills = ["review"]                  # /skill enable <name> 전까지 스킬 숨기기

[permissions]
mode  = "ask"                                # 규칙이 일치하지 않을 때 작성자 폴백: ask|allow|deny
deny  = ["Bash(rm -rf*)", "Bash(git push*)"] # 모든 모드에서 하드 차단
allow = ["Bash(go test:*)"]                  # 프롬프트 표시 안 함

[sandbox]
# workspace_root = ""          # 파일 작성자 제한 위치; 비어 있으면 현재 디렉터리
# allow_write    = ["/tmp"]    # write_file/edit_file/multi_edit/move_file이 접근할 수 있는 추가 디렉터리
# forbid_read    = ["${HOME}/.ssh"]   # 에이전트가 읽거나 나열해서는 안 되는 경로

[serve]
auth_mode = "none"             # none|token|password; localhost를 벗어나 바인딩하기 전에 인증 사용
# token = ""                   # 선택적 고정 토큰; 빈 토큰 모드는 시작 시 생성
# password_hash = ""           # patcode serve --hash-password --password '...'로 생성된 bcrypt 해시
# behind_proxy = false         # 신뢰할 수 있는 리버스 프록시 뒤에서만 true

[[plugins]]
name    = "example"
command = "patty-plugin-example"
startup_timeout_seconds = 60   # 선택적 initialize + tools/list 상한
call_timeout_seconds = 600   # 선택적 서버별 MCP 호출 타임아웃
tool_timeout_seconds = { "generate_video" = 1800 }   # 선택적 원시 MCP 도구 이름
```

전체 스키마와 각 필드의 계약은 [`SPEC.md` §5](../reference/SPEC.md#5-configuration-toml)를 참조하세요.

설치된 MCP 서버와 프로젝트에서 구성한 MCP 서버는 도구별 신뢰 목록이 필요하지 않습니다. 전용 이중 모델 Planner는 서버가 `readOnlyHint`를 생략하더라도 모든 비파괴적 MCP 도구를 사용할 수 있습니다. 엄격한 읽기 전용 하위 에이전트는 여전히 `readOnlyHint: true`가 있고 `destructiveHint`가 없어야 합니다.

`[agent].plan_mode_read_only_commands`는 구성 왕복(round trip)을 위해 유지되지만, 주요 Plan 워크플로에는 더 이상 별도의 bash 허용 목록이나 신뢰 프롬프트가 없습니다. Bash 분류와 승인은 Plan 및 Standard 모드에서 동일한 Permissions 규칙을 사용하며, Sandbox는 파일 시스템, 프로세스, 네트워크 경계로 남습니다. 전용 플래너 및 읽기 전용 하위 에이전트 실행기는 자체적인 엄격한 읽기 전용 도구 레지스트리와 포그라운드 명령 분류기를 유지합니다.

### 환경 변수

대부분의 일상 설정은 앞서 설명한 `config.toml` 또는 전역 Patty Code `.env`에 들어갑니다. 아래의 변수는 프로세스 수준의 고급 스위치이므로 Patty Code를 실행하기 전에 설정하세요. 프로젝트 `.env` 파일은 Patty Code 제어 변수의 런타임 소스가 아닙니다.

### CLI 텔레메트리(telemetry)

CLI는 하루에 한 번의 익명 활성 설치 핑(ping)과 제한된 콘텐츠 없는 이벤트 카운터를 벤더 텔레메트리 엔드포인트로 보낼 수 있습니다. 사용자 전역 정책은 다음과 같이 구성합니다:

```bash
patcode config telemetry          # 현재 적용 중인 모드 출력
patcode config telemetry auto     # 기본값: 로컬 대화형 TTY만
patcode config telemetry on       # 로컬 헤들리스 `patcode run`도 허용
patcode config telemetry off      # 비활성화 및 대기 중인 카운터 파일 삭제
```

자격을 갖춘 최초의 릴리스 빌드 대화형 세션에서 Patty Code는 정확한 데이터 범위를 설명하고 텔레메트리 요청 전에 한 번만 확인을 요청합니다. 프롬프트는 `[Y/n]`이며, Enter, `y` 또는 `yes`를 누르면 `auto`가 저장되고, `n` 또는 `no`를 누르면 `off`가 저장되고 대기 중인 카운터가 삭제됩니다. 선택이 저장된 후에는 활성화된 보고가 조용히 진행되며 프롬프트가 다시 표시되지 않습니다. 기본 설정을 저장할 수 없으면 아무것도 업로드되지 않습니다.

보고는 CI, 개발 빌드, 그리고 `DO_NOT_TRACK`가 설정되었거나 `PATTY_TELEMETRY=0`인 경우 항상 비활성화됩니다. `auto`에서는 리다이렉트/파이프된 세션이나 기타 비대화형 세션이 보고하지 않습니다. 아직 선택이 저장되지 않은 경우 이러한 부적격 세션은 프롬프트를 표시하지도 않고 보고하지도 않습니다. 동의 후의 네트워크 오류는 조용히 처리되며 stdout, stderr, 프로세스 종료 코드를 절대 변경하지 않습니다. 전송되지 않은 카운터는 이후 호출을 위해 제한된 로컬 큐에 남습니다.

핑에는 전용 무작위 128비트 CLI 설치 ID, CLI 버전, OS, 아키텍처, `cli` 표면 마커가 포함됩니다. 카운터 배치는 일일 활성 설치 중복 제거에 동일한 ID를 사용하며, CLI 모드/프로필, 권한/세션 모드, 턴 지연 시간, 완료 사유, 캐시 적중 범위, 일반 Provider/도구 오류 클래스, 압축, 복구 카운터, 정규화된 UI 언어 등의 고정 버킷만 포함합니다. 이 ID는 데스크톱 설치 ID와 분리되며 계정, 하드웨어, 저장소 또는 세션 식별자가 아닙니다.

Patty Code는 프롬프트, 답변, patty code, 도구 이름/인수/출력, 경로, 저장소/브랜치, 세션 ID, 정확한 토큰 또는 비용 값, Provider/모델 이름, base URL, 환경 변수를 절대 업로드하지 않습니다.

### CLI 크래시 리포트(crash report)

CLI 진입점까지 도달한 처리되지 않은 Go 패닉(panic)은 `<Patty Code home>/cli-crash-reports` 아래에 개인정보가 제거된 리포트로 로컬 저장됩니다. Patty Code는 소유자 전용 권한으로 최대 10개의 파일을 유지합니다. 패닉 값은 절대 직렬화되지 않습니다. 절대 소스 경로는 `<path>/<file>.go:<line>`이 되고 함수 인수는 제거되며, 동일한 비밀값, 토큰, 이메일, 긴 식별자 스크러버가 저장 시와 전송 직전에 모두 실행됩니다.

크래시 리포트는 자동으로 업로드되지 않습니다. 다음과 같이 검토하고 관리하세요:

```bash
patcode report                 # 최신 리포트 미리보기; TTY에서 전송 전 확인
patcode report list            # 로컬 리포트 목록
patcode report show [ID]       # 전송 없이 미리보기
patcode report send [ID]       # 명시적 전송; 성공 후에만 로컬 삭제
patcode report delete [ID]     # 전송 없이 삭제
```

파이프되거나 리다이렉트된 `patcode report` 호출은 미리보기만 수행하며 프롬프트를 표시하거나 전송하지 않습니다. CLI 텔레메트리 설정은 이렇게 별도로 검토되는 리포트를 자동 전송하거나 자동 삭제하지 않습니다. 래핑되지 않은 백그라운드 고루틴의 런타임 치명적 오류, 운영 체제 종료, 패닉은 Go가 복구할 수 없으므로 이 로컬 리포트를 생성하지 않습니다.

## Serve 웹 프론트엔드

`patcode serve`는 브라우저 UI 뒤에서 동일한 로컬 엔진을 시작합니다. 데스크톱 앱을 설치하지 않고 데스크톱 스타일의 화면을 원할 때, 터널을 통해 원격 개발 머신에서 Patty Code를 실행할 때, 또는 실행 중인 세션을 공유 가능한 뷰로 보고 싶을 때 사용하세요.

```bash
cd your-project
patcode serve
# http://127.0.0.1:8787 열기
```

기본적으로 `127.0.0.1:8787`에서 `auth_mode = "none"`으로 수신 대기합니다. 로컬 전용 사용에는 이 기본값을 유지하세요. 루프백을 벗어나 바인딩하거나, 터널로 노출하거나, 리버스 프록시 뒤에 두는 경우에는 URL을 공유하기 전에 인증을 활성화하세요:

```bash
patcode serve --auth token
patcode serve --addr 0.0.0.0:8787 --auth token
patcode serve --auth password --password 'temporary-password'
```

토큰 모드는 `?token=...`이 포함된 공유 URL을 출력합니다. 안정적인 토큰을 재사용하려면 `--token`을 전달하거나 `[serve].token`을 설정하세요. 비밀번호 모드는 시작 시 `--password` 또는 저장된 bcrypt 해시가 필요합니다:

```bash
patcode serve --hash-password --password 'strong-password'

# <Patty Code home>/config.toml
[serve]
auth_mode = "password" # none|token|password
password_hash = "$2a$12$..."
behind_proxy = true    # 신뢰할 수 있는 리버스 프록시 뒤에서만
```

웹 UI는 채팅, 도구 승인, 세션 기록, 되감기/포크/요약, 모델 및 reasoning 강도 컨트롤, Goal, `todo_write` 도구가 공급하는 실시간 todo 패널, 확장 상태/카드/폼/알림 화면, 구성 시 제공자 잔액을 제공합니다. 확장 호스팅 제공자는 모델 선택기에 나타납니다. 유휴 상태에서 `/reload`를 실행하면 Serve를 재시작하지 않고 확장 사이드카와 런타임 세대를 원자적으로 다시 로드합니다. 일회성 실행에는 `--model`, `--max-steps` 또는 `--resume`을 사용하며, 그 외에는 `serve`가 사용자 전역 `default_model`을 사용합니다.

선택한 Provider에 저장된 API 키가 없어도 루프백 바인딩된 Serve는 브라우저가 연결되기 전에 실패하지 않고 시작되어 Provider 설정 페이지를 표시합니다. 인증 후 그 자리에서 키를 입력하면 Patty Code가 이 호스트의 전역 자격 증명 파일에 제한된 권한으로 키를 기록하고, 동일한 프로세스에서 활성 컨트롤러를 재구축한 뒤 일반 UI를 엽니다. 자격 증명 작성 엔드포인트는 루프백이 아닌 수신 대기자에서는 비활성화됩니다. 원격 SSH 창에서 "이 호스트"는 SSH 터널을 통해 도달하는 원격 호스트를 의미하며, 키는 데스크톱 머신에서 복사되지 않습니다.

## ACP를 통한 에디터 통합

`patcode acp`는 Patty Code를 ACP v1 stdio 에이전트로 노출하여 에디터 및 기타 호스트 클라이언트에서 사용할 수 있게 합니다. 전용 **[ACP 에디터 통합](./ACP.md)** 가이드에서 시작, 기능 협상, 세션 수명 주기, 독립적인 모델/작업/협업/승인 컨트롤, 클라이언트 파일 시스템 및 터미널 기능, MCP 서버, 권한 요청, Patty Code의 턴 중간 스티어링 확장을 다룹니다.

## 원격 SSH

원격 모듈은 원격 호스트에서 Patty Code를 실행하고 사용자 고유의 SSH 연결로 접근합니다 — VS Code Remote-SSH 방식입니다. 원격 호스트에서 지속적인 헤들리스 `patcode serve`를 부트스트랩하고, 로컬 루프백 포트를 그쪽으로 포워딩하며, 해당 터널을 통해 기존 serve 웹 클라이언트를 엽니다. 에이전트, 도구, 파일은 모두 원격 호스트에 완전한 충실도로 존재하며, 손실이 있는 파일 프록시를 거치지 않습니다. V1은 Linux 및 macOS 원격 호스트를 지원합니다.

호스트는 `config.toml`의 사용자 전역 `[remote]` 섹션에 저장됩니다. `[secrets]`와 마찬가지로 프로젝트 `patty.toml`은 원격 호스트를 주입하거나 재정의할 수 없습니다. 즉, 복제된 저장소가 Patty Code가 SSH 연결을 여는 위치를 조종할 수 없습니다. 자격 증명은 제공자 관용구를 따릅니다: 호스트는 환경 변수 이름(`passphrase_env`, `password_env`)을 지정하며, 그 값은 Patty Code 전역 `.env`에 저장됩니다. 키 자료 자체는 절대 저장되지 않으며, `identity_file`은 경로입니다.

```toml
[remote]
[[remote.hosts]]
name          = "gpu-box"
host          = "203.0.113.7"
user          = "dev"
identity_file = "~/.ssh/id_ed25519"
workspace     = "~/projects/app"
serve_install = "auto"            # Remote CLI: auto | npm | upload | never

[[remote.hosts.forwards]]
type   = "local"                  # local (-L) | remote (-R)
bind   = "127.0.0.1:5432"
target = "127.0.0.1:5432"
```

CLI:

```bash
patcode remote add gpu-box dev@203.0.113.7 --workspace '~/projects/app'
patcode remote import --all              # 별칭 가져오기; 연결 시 ssh -G가 Include/Match 규칙을 해석
patcode remote test gpu-box              # 연결 + 인증 + 호스트 키 확인
patcode remote connect gpu-box --open    # serve 부트스트랩, 터널, URL 열기
patcode remote serve status gpu-box
patcode remote fs ls gpu-box:'~/projects/app'
```

`use_ssh_config`가 활성화된 호스트는 로컬 OpenSSH의 `ssh -G`를 통해 최종 유효 구성을 해석합니다. 여기에는 `Include`, 와일드카드 `Host`, `Match`(`Match exec` 포함), 반복된 `IdentityFile`, `ProxyJump`, `IdentitiesOnly`가 포함됩니다. 가져오기는 오래된 스냅샷 대신 원래 별칭을 저장합니다.

`connect`는 포그라운드 슈퍼바이저입니다(`ssh -N`에 serve 부트스트랩을 더한 것): 터널과 구성된 포워드를 유지하고, 링크가 끊어지면 지수 백오프로 자동 재연결하며, 재연결 시 포워드를 다시 연결합니다. Ctrl-C는 로컬 측만 끊습니다 — 원격 serve는 계속 실행되므로 다음 `connect`가 이를 재사용합니다. V1에는 백그라운드 데몬이 없습니다.

호스트 키는 OpenSSH `~/.ssh/known_hosts`(읽기 전용)와 Patty Code가 관리하는 `~/.patty/remote/known_hosts`에 대해 검증됩니다. 처음 보는 키는 최초 사용 시 신뢰(trust-on-first-use) 프롬프트를 표시하고 관리 파일에 기록합니다. 기록된 키와 모순되는 키는 문제의 줄을 명명하는 하드 오류가 되며 자동으로 수락되지 않습니다.

원격 측 상태는 원격 호스트의 `~/.patty/remote/` 아래에 있습니다: `serve-<workspace-slug>.json`(pid, 바인딩된 루프백 주소, 워크스페이스), `serve-<slug>.token`(0600; `--token-file`로 serve에 전달되어 `ps`에 절대 나타나지 않는 인증 토큰), `serve-<slug>.log`.

데스크톱 앱에서는 **Settings → Remote SSH**에서 호스트를 관리한 다음, 상태 표시줄 칩이나 호스트 행의 **Remote explorer** 버튼으로 SFTP를 통해 파일을 탐색·편집하고 포트 포워드를 관리하며 원격 워크스페이스를 시작/엽니다. 워크스페이스를 열면 VS Code Remote SSH 창과 유사한 별도의 네이티브 Patty Code 창이 생성됩니다. 기본 창이 SSH 터널을 소유하며, 원격 창은 격리된 경량 셸로 로컬 대화 세션을 복원하거나 획득하지 않습니다. 원격 웹 페이지는 **원격** 호스트의 제공자 구성과 API 키를 사용합니다 — 데스크톱은 자체 제공자를 원격 호스트에 절대 노출하지 않습니다. 해당 호스트에 선택한 Provider의 API 키가 없으면 창은 먼저 인증된 설정 페이지를 표시하고, 키를 원격 Patty Code 자격 증명 파일에만 저장하며, 원격 Serve 프로세스를 재시작하지 않고 Provider를 활성화합니다. 일시적인 SSH 중단은 원격 창을 열린 상태로 유지합니다. 데스크톱은 백그라운드에서 재연결하고 루프백 포워드를 다시 연결하며 복구된 Serve에 맞춰 창을 다시 로드합니다. 인증 또는 호스트 키 오류는 종결적이며, 사용할 수 없는 원격 창을 대신 닫습니다.

## 사용자 지정 OpenAI 호환 제공자

데스크톱 앱에서 OpenAI 호환 채팅 API 또는 Anthropic 호환 Messages API를 말하는 프록시, 애그리게이터, 자체 호스팅 서비스에 대해 **Settings → Model → Access → Add model service → Custom provider**를 엽니다.

일반적인 제공자는 대신 **Add model service → Recommended preset**을 선택하세요. 공식 DeepSeek 서비스는 기본적으로 특별히 적응된 OpenAI Chat Completions 경로를 계속 사용하며, Anthropic Messages 호환이 필요한 경우에만 선택적 **DeepSeek Anthropic** 사전 설정을 추가하세요. 두 항목은 서로를 대체하지 않습니다. Patty Code는 DeepSeek, LongCat, OpenCode Go, OpenCode Go Anthropic, OpenCode Zen Anthropic, Vercel AI Gateway, HuggingFace Router, NVIDIA NIM, KiloCode, Ollama Cloud, 그리고 일부 OpenAI 호환 서비스에 대해 편집 가능한 사용자 지정 제공자 항목을 미리 채울 수 있습니다. 사전 설정 경로는 보통 제공자 API 키만 필요합니다. 키 값은 Patty Code home `.env`에 저장되고, `config.toml`에는 엔드포인트, 모델 목록, 키 환경 변수 이름, 컨텍스트 창, 비전 모델 메타데이터, 필요한 경우 Anthropic 호환 Bearer 인증, 그리고 해당 사전 설정이 노출하는 모델별 reasoning 재정의가 저장됩니다. 사전 설정을 추가한 뒤 모델, 헤더, 엔드포인트 또는 호환성 설정을 바꾸고 싶다면 해당 제공자 카드를 열어 수정하세요.

**API address**에는 표준 채팅 경로를 수신할 제공자 엔드포인트를 입력합니다. 이 모드에서 Patty Code는 다음 위치로 채팅 요청을 미리 보고 전송합니다:

```text
<API address>/chat/completions
```

**Full URL**은 서비스가 완전한 요청 URL을 제공할 때 활성화합니다. 예: `https://gateway.example.com/v1/chat/completions`. 그러면 Patty Code는 채팅 요청을 해당 URL로 직접 보내고 `/chat/completions`를 추가하지 않습니다. 필드 아래의 미리보기는 사용될 정확한 요청 URL을 보여줍니다.

모델 검색은 API 주소를 사용하여 `/models` 및 `/v1/models` 같은 가능성 있는 모델 목록 URL을 시도합니다. 게이트웨이가 별도의 모델 목록 엔드포인트를 요구하면 **Compatibility settings**를 열고 `models_url`을 설정하세요. 예: `https://gateway.example.com/v1/models`. 검색을 사용할 수 없으면 모델 목록을 수동으로 채우세요.

**Full URL**은 여전히 OpenAI 호환 채팅 요청 본문을 사용합니다. 요청 스키마를 OpenAI Responses API로 전환하지 않습니다.

### 호환성 설정

**Compatibility settings(보통 변경하지 않음)** 섹션은 인증, 모델 목록 엔드포인트 또는 reasoning/사고 요청 형태가 일반적인 OpenAI 호환 기본값과 다른 게이트웨이를 위한 것입니다. 제공자 문서나 프록시 오류가 달리 알려주지 않는 한 이 필드를 기본값으로 두세요. 일부 코딩 플랜 엔드포인트 같은 Anthropic 호환 서비스의 경우 저장하기 전에 연결 프로토콜로 **Anthropic-compatible**을 선택하세요.

| 필드 | 제어 대상 | 변경 시점 |
| --- | --- | --- |
| `api_key_env` | 이 제공자의 API 키에 사용되는 환경 변수 이름. 데스크톱에서 저장한 키 값은 Patty Code home `.env`에 이 이름으로 저장되며, TOML 구성에는 이름만 저장됩니다. | 여러 제공자가 서로 다른 키를 필요로 할 때 변경하거나, API 키가 필요 없는 서비스에는 비워 두세요. |
| `models_url` | 모델 검색에만 사용되는 URL. 채팅 요청은 여전히 위의 API 주소 또는 Full URL을 사용합니다. | 게이트웨이가 모델 목록을 `/models` 또는 `/v1/models`가 아닌 곳에 노출할 때 설정하세요. |
| Extra request headers | 한 줄에 `Header: value` 형식의 정적 HTTP 헤더. | `HTTP-Referer`, `X-Title` 또는 유사한 사이트 헤더를 요구하는 OpenRouter 같은 게이트웨이에 사용하세요. Bearer/API 키는 여기에 중복하지 말고 키 필드에 두세요. |
| Extra request body | 채팅 요청 본문의 최상위에 병합되는 JSON 객체. | `{"enable_thinking": true}` 같은 제공자별 플래그에만 사용하세요. Patty Code는 `model`, `messages`, `tools`, `stream`, `thinking` 같은 핵심 필드를 계속 소유하며 null 값은 거부됩니다. |
| Authorization: Bearer | Anthropic 호환 제공자의 경우 저장된 API 키를 `x-api-key` 대신 `Authorization: Bearer <key>`로 전송합니다. | Vercel AI Gateway처럼 게이트웨이가 Bearer 인증을 문서화한 경우에만 활성화하세요. |
| Model capability mode | 이 제공자에 Patty Code가 사용할 patty code 요청 프로토콜. | 게이트웨이가 잘못 감지되거나 모델 문서가 특정 patty code 형식을 요구하지 않는 한 **Auto-detect**를 유지하세요. |
| Thinking override | `thinking.type`에 대한 제공자별 재정의. | 백엔드가 `enabled`, `disabled` 또는 `adaptive`를 문서화하지 않는 한 **Auto**를 유지하세요. 지원되지 않는 값은 일부 OpenAI 호환 게이트웨이가 요청을 거부하게 만들 수 있습니다. |
| Balance URL | 지갑/잔액 조회용 선택적 엔드포인트. | 제공자가 잔액 엔드포인트를 노출하고 데스크톱 상태 표시줄에 표시하려 할 때 설정하세요. |
| Context window | 자동 컨텍스트 정리에 Patty Code가 사용하는 제공자 전체 토큰 예산. `0`은 자동 압축을 비활성화합니다. | 제공자의 모델 컨텍스트 제한으로 설정하고, 선택한 모델이 서로 다르면 아래의 모델별 재정의를 사용하세요. |

각 선택 모델에는 선택적 **Context window** 입력도 있습니다. 비워 두면 제공자 전체 값을 상속하고, 양수 토큰 수를 입력하면 이 모델에 대해 해당 값을 재정의합니다. 이렇게 하면 긴 컨텍스트 모델의 조기 압축과 동일한 엔드포인트를 공유하는 짧은 컨텍스트 모델의 제공자 오류를 피할 수 있습니다. 모델 문서의 컨텍스트 창 제한을 사용하세요(최대 출력 토큰이 아님). 예를 들어 128K는 보통 `128000`을 의미하며, 제공자가 `131072`를 문서화하면 그 정확한 값을 사용하세요. 16384 미만의 값은 잦은 압축과 캐시 적중률 저하를 유발할 수 있으므로 비차단 경고를 표시합니다.

모델 기능 모드 옵션:

| 옵션 | 효과 |
| --- | --- |
| Auto-detect(권장) | Patty Code가 모델 기능 메타데이터와 엔드포인트 감지에서 요청 형태를 선택합니다. |
| DeepSeek thinking | `thinking.type` 및 DeepSeek 지원 reasoning 깊이를 포함한 DeepSeek 스타일 사고 제어를 사용합니다. |
| OpenAI reasoning | 표준 OpenAI 호환 `reasoning_effort` 수준을 사용합니다. |
| Plain chat | reasoning 또는 사고 제어 필드를 전송하지 않습니다. reasoning 매개변수를 거부하는 텍스트 전용 프록시에 사용하세요. |

사고 재정의 옵션:

| 옵션 | 효과 |
| --- | --- |
| Auto(제공자 기본값) | 명시적인 제공자 수준 `thinking` 재정의를 쓰지 않습니다. Patty Code는 제공자/모델 기본 동작을 사용합니다. |
| Enabled | 호환 제공자에 `thinking.type = "enabled"`를 전송합니다. |
| Disabled | 호환 제공자에 `thinking.type = "disabled"`를 전송합니다. DeepSeek 스타일 제공자에서는 reasoning 깊이 힌트 전송도 피합니다. |
| Adaptive(자체 조정) | 적응형 사고를 명시적으로 문서화한 제공자에만 `thinking.type = "adaptive"`를 전송하거나 유지합니다. |

일부 OpenAI 호환 게이트웨이는 비표준 최상위 요청 본문 필드를 요구합니다. 제공자 항목에 `extra_body`로 추가하세요:

```toml
[[providers]]
name        = "custom-gateway"
kind        = "openai"
base_url    = "https://gateway.example.com/v1"
models      = ["example-model"]
api_key_env = "EXAMPLE_API_KEY"
extra_body  = { enable_thinking = true }
```

`extra_body`는 채팅 JSON 요청 본문에 병합됩니다. Patty Code는 `model`, `messages`, `tools`, `stream`, `thinking` 같은 핵심 필드를 자체 제어 하에 유지합니다.

## 데스크톱 훅

데스크톱 훅은 `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PreCompact` 같은 수명 주기 이벤트에서 로컬 명령을 실행합니다. 성공적인 `SessionStart` 훅은 stdout에 일반 텍스트를 쓰거나 `hookSpecificOutput.additionalContext`가 있는 JSON을 반환할 수 있으며, Patty Code는 다음 실제 사용자 턴에 `<hook-context event="SessionStart">...</hook-context>`로 해당 텍스트를 한 번 주입합니다. 이는 Superpowers 스타일 시작 지침을 포함한 플러그인 또는 워크플로 부트스트랩 컨텍스트를 위해 설계된 것으로, 해당 워크플로를 Patty Code의 시스템 프롬프트에 굽지 않기 위함입니다.

플러그인 패키지는 `hooks/session-start-codex` 또는 플러그인 루트의 `CLAUDE.md`를 통해 이 시작 컨텍스트를 제공할 수 있습니다. Claude 스타일 `.claude/settings.json` 명령 훅도 일치하는 Patty Code 훅 이벤트에 매핑됩니다.

주입된 훅 컨텍스트는 동적인 현재 턴 컨텍스트입니다. 안정적인 시스템 프롬프트, 메모리 접두사 또는 도구 스키마를 변경하지 않지만, 동적 콘텐츠는 해당 턴의 캐시 재사용을 줄일 수 있습니다. 데스크톱 훅 스키마와 로딩 모델은 이 가이드의 데스크톱 훅 섹션에 설명되어 있습니다.

## 키보드 단축키

단축키는 사용자가 사용 중인 화면에서 작동하는 키를 보통 찾기 때문에 클라이언트별로 문서화됩니다. 데스크톱은 Plan 토글을 유지하고, CLI는 `Shift+Tab`으로 Ask, Auto, Plan을 순환합니다. 데스크톱은 기본적으로 macOS에서 `Cmd+Y`, 그 외에는 `Ctrl+Y`를 YOLO에 사용합니다. Windows/Linux에서 YOLO가 다시 바인딩되면 `Ctrl+Y`가 표준 컴포저 다시 실행 폴백이 됩니다. 데스크톱 붙여넣기는 플랫폼 붙여넣기 키에 유지되며, CLI에서는 터미널 네이티브 텍스트 붙여넣기와 애플리케이션 소유 이미지 붙여넣기가 별도의 단축키를 사용합니다.

`[ui].shortcut_layout`은 이전 구성에서 계속 허용되지만, 아래의 단축키 동작은 레이아웃에 걸쳐 통합됩니다.

CLI/TUI 텍스트 입력에서 `[ui].cursor_shape`는 `underline`, `block` 또는 `bar`를 허용합니다. 기본값은 `bar`이며, 혼합 언어 입력에서 전각 CJK 문자를 가리지 않고 쉽게 찾을 수 있습니다. 전통적인 터미널 커서를 원하면 `block`, 낮은 프로필 커서를 원하면 `underline`로 설정하세요. 이 설정은 데스크톱이나 웹 텍스트 필드를 변경하지 않습니다.

### 데스크톱 GUI

데스크톱 단축키는 **Settings → Shortcuts**에서 관리합니다. 구성 가능한 행을 선택하고 새 키 조합을 누르면 Patty Code가 데스크톱 앱용으로 저장합니다. Undo, Redo 같은 표준 편집 단축키는 WebView의 네이티브 텍스트 기록이 해당 플랫폼 코드를 사용하므로 잠긴 행으로 표시됩니다. 충돌하는 바인딩은 거부되어 하나의 단축키가 두 동작을 절대 트리거하지 않습니다. `?`를 누르거나 주제 표시줄의 도움말 버튼을 사용해 단축키 시트를 열 수 있으며, 이 시트는 동일한 단축키 레지스트리에서 생성되므로 사용자 지정 바인딩을 반영합니다.

전역 단축키:

| 키 또는 컨트롤 | 기능 | 참고 |
| --- | --- | --- |
| macOS에서 `Cmd+K`, Windows/Linux에서 `Ctrl+K` | 명령 팔레트 토글 | 팔레트는 열릴 때 검색에 포커스를 두며, `Esc`로 닫습니다. |
| macOS에서 `Cmd+,`, Windows/Linux에서 `Ctrl+,` | Settings 열기 | 설정의 **Shortcuts**로 데스크톱 바인딩을 사용자 지정하세요. |
| macOS에서 `Cmd+W`, Windows/Linux에서 `Ctrl+W` | 활성 상위 탭 닫기 | 마지막 탭은 일반 닫기 탭 가드로 유지됩니다. |
| `Cmd+B` / `Ctrl+B` | 왼쪽 사이드바 표시/숨기기 | 사이드바 토글 클릭과 동일한 동작입니다. |
| `Cmd+Shift+B` / `Ctrl+Shift+B` | 가장 최근 셸 출력 펼치기/접기 | 접힌 셸 출력 힌트 클릭과 동일한 동작입니다. |
| macOS에서 `Cmd+1`-`Cmd+9`, 그 외 `Ctrl+1`-`Ctrl+9` | 사이드바의 해당 표시 채팅으로 이동 | `Cmd`/`Ctrl`을 잠시 누르면 번호 배지가 표시됩니다. 동일한 키를 이미 사용하는 기존 사용자 지정 단축키가 우선합니다. |
| macOS에서 `Cmd++`, `Cmd+-`, `Cmd+0`; 그 외 `Ctrl++`, `Ctrl+-`, `Ctrl+0` | 텍스트 크기 증가, 감소 또는 초기화 | 해당 방식으로 보고하는 키보드에서는 `=`가 더하기 키로 허용됩니다. |
| `?` | 키보드 단축키 시트 열기 | 시트는 현재 적용 중인 데스크톱 바인딩을 보여줍니다. |

컴포저 단축키:

| 키 또는 컨트롤 | 기능 | 참고 |
| --- | --- | --- |
| `Enter` | 현재 메시지 전송 | IME 조합 확인은 그대로 둡니다. |
| `Shift+Enter` | 새 줄 삽입 | 컴포저는 포커스를 유지합니다. |
| `Shift+Tab` | Plan 켜기/끄기 | Plan은 워크플로 지침을 변경합니다. 내장 작성자는 활성 Ask/Auto/YOLO 및 Sandbox 경계를 유지하고, MCP 작성자/파괴적 대상은 전체 계획 단계 동안 하드 차단됩니다. |
| macOS에서 `Cmd+Z`, Windows/Linux에서 `Ctrl+Z` | 최신 컴포저 편집 실행 취소 | 네이티브 입력은 WebView 기록에 남고, Patty Code가 관리하는 붙여넣기, 잘라내기, 접힌 블록, 구조화된 토큰은 완전한 트랜잭션으로 복원됩니다. |
| macOS에서 `Cmd+Shift+Z`, Windows/Linux에서 `Ctrl+Shift+Z` | 최신 컴포저 편집 다시 실행 | Windows/Linux에서 YOLO 단축키가 다시 바인딩된 후에는 `Ctrl+Y`도 허용됩니다. |
| `Cmd+Y` / `Ctrl+Y`(기본값) | YOLO 켜기/끄기 | YOLO를 끄면 알려진 경우 이전 Ask/Auto 기본값을 복원합니다. 현재 바인딩은 **Settings → Shortcuts**에 표시됩니다. |
| macOS에서 `Cmd+V`, Windows/Linux에서 `Ctrl+V` | 클립보드 콘텐츠 붙여넣기 | 클립보드 이미지는 첨부되며, 이미지를 컴포저에 끌어다 놓을 수도 있습니다. |
| 프롬프트 경계에서 일반 `Up` / `Down` | 이전 또는 이후 제출 프롬프트 불러오기 | 수정된 화살표와 네이티브 텍스트 탐색은 텍스트 영역에 유지됩니다. |
| 턴 실행 중 `Esc` | 실행 중인 턴 취소 | 턴이 아직 응답을 생성하지 않았으면 초안이 복원됩니다. |

메뉴 및 컨트롤:

| 키 또는 컨트롤 | 기능 | 참고 |
| --- | --- | --- |
| 슬래시, `@` 또는 과거 채팅 메뉴에서 `Up` / `Down` | 강조 표시된 항목 이동 | 과거 채팅 검색은 동일한 탐색 키를 사용합니다. |
| 해당 메뉴에서 `Enter` / `Tab` | 강조 표시된 항목 수락 | 디렉터리형 항목은 다음 수준을 위해 메뉴를 열어 둘 수 있습니다. |
| 해당 메뉴에서 `Esc` | 현재 메뉴 닫기 또는 과거 채팅 검색에서 복귀 | 메뉴가 닫힌 후 일반 입력이 계속됩니다. |
| Ask / Auto / YOLO 승인 컨트롤 | 도구 승인 자세를 직접 선택 | 이 컨트롤 클릭은 키보드 단축키의 영향을 받지 않습니다. |
| 도구 승인 카드 | `Left` / `Right`, `Enter`, `1`-`4`, `Esc` | 강조 표시된 동작 이동, 확인, 번호가 매겨진 동작 선택 또는 거부. 기본 강조 동작은 Allow once입니다. |
| Plan 승인 카드 | `Left` / `Right`, `Enter`, `1`-`3`, `Esc` | Revise plan, Start execution, Exit plan 사이 이동. 기본 강조 동작은 Start execution입니다. |
| Plan 컨트롤 | Plan 켜기/끄기 | `Shift+Tab`과 동일한 모드입니다. |
| 협업 메뉴의 Goal 항목 | Goal 시작, 보기 또는 지우기 | Goal은 어떤 키보드 순환에도 없습니다. |

### CLI / TUI

컴포저는 기본적으로 테마 색상의 상단/하단 테두리와 가는 막대 커서를 사용합니다. 긴 초안은 사용 가능한 최대 높이까지 커집니다. 넘치면 컴포저 내부의 휠 이벤트가 삽입 커서를 움직이지 않고 초안을 스크롤하며, 트랜스크립트의 휠 이벤트는 계속 대화를 스크롤합니다. 배경 모드 선택은 `/theme auto|light|dark`, 명명된 강조 팔레트 선택은 `/theme <style>`을 사용하세요(팔레트 목록은 `/theme`만 입력).

반응형 푸터는 활성 Ask/Auto/Plan 또는 YOLO 자세와 현재 상호작용 상태를 왼쪽에 유지합니다. 넓은 터미널에서는 모델, 강도, 작업 모드가 오른쪽에 함께 유지되며, 두 번째 행에 사용 가능한 Git 신원, 캐시 적중률, 컨텍스트 사용량, 압축 여유, 작업, 잔액이 표시됩니다. `ready`는 유휴 컴포저 상태이며 모델 상태 확인이 아닙니다. 선택기, 승인, 이미지 붙여넣기, 셸 모드 및 기타 활성 상호작용이 이를 대체합니다. 좁은 터미널에서는 전체 그룹을 이동, 줄바꿈 또는 압축하며, 레이블과 표시되는 작업 모드 값은 `/language`를 따르고 `/work-mode` 명령 인수는 안정적인 영어 식별자로 유지됩니다.

채팅 및 트랜스크립트 단축키:

| 키 또는 명령 | 기능 | 참고 |
| --- | --- | --- |
| `Enter` | 현재 메시지 전송 | 턴 실행 중에는 비어 있지 않은 입력이 후속 피드백으로 큐에 들어갑니다. |
| `Shift+Enter`, `Alt+Enter` 또는 `Ctrl+J` | 새 줄 삽입 | 일반 `Enter`는 전송/확인용으로 예약됩니다. |
| 유휴 상태에서 일반 `Up` / `Down` | 이전 또는 이후 제출 프롬프트 불러오기 | 실행 중인 턴에서는 같은 키가 큐에 있는 후속 피드백을 탐색합니다. |
| `PageUp` / `PageDown` | 트랜스크립트 스크롤 | 현재 채팅 상태와 관계없이 작동합니다. |
| `Ctrl+Home` / `Ctrl+End` | 트랜스크립트 맨 위 또는 맨 아래로 이동 | 긴 도구 출력 후에 유용합니다. |
| `Ctrl+L` 또는 `/cls` | 표시된 트랜스크립트만 지우기 | LLM 컨텍스트, 세션 파일, 도구, 메모리, 플러그인은 계속 로드됩니다. 대화 컨텍스트를 버리려면 `/clear`를 사용하세요. |
| `Esc` | 현재 동작에서 빠져나오기 | 응답 전에 방금 제출한 턴을 취소하고, 실행 중인 턴을 취소하거나 비어 있지 않은 입력을 지웁니다. |
| 빈 유휴 컴포저에서 `Esc` 두 번 | 되감기 선택기 열기 | `/rewind`와 동일한 진입점입니다. |
| 트랜스크립트 텍스트 선택 | 트랜스크립트 텍스트 복사 | 앱 내 드래그를 놓으면 로컬 세션에서 검증된 네이티브 클립보드 경로로 기록됩니다(macOS의 `pbcopy`, Linux의 사용 가능한 Wayland/X11 도구 또는 Windows 클립보드). SSH에서는 OSC 52로 폴백하고 네이티브 성공으로 주장하는 대신 폴백임을 표시합니다. `Ctrl+C`/`Super+C`/`Meta+C` 또는 활성 선택 영역 우클릭으로 다시 복사합니다. |
| 컴포저 텍스트 선택 | 초안 텍스트 선택, 복사 또는 교체 | 앱 내 드래그를 놓으면 트랜스크립트 텍스트와 동일한 검증된 클립보드 경로로 선택 영역을 복사합니다. 입력하거나 붙여넣으면 선택 영역이 교체되고, 화살표 키는 선택을 축소합니다. |
| 활성 선택 없이 우클릭 | 클립보드 텍스트를 로컬에 붙여넣기 | 앱 내 마우스 캡처가 켜진 로컬 세션에서 Patty Code는 텍스트만 읽고 일반 bracketed-paste 처리를 거칩니다. SSH에서는 원격 프로세스가 로컬 클립보드를 읽을 수 없으므로 터미널 붙여넣기 단축키를 사용하세요. `/mouse`는 터미널의 네이티브 우클릭 메뉴를 복원합니다. 활성 선택 영역이 있는 우클릭은 여전히 해당 선택을 복사합니다. |
| `/mouse` | 앱 내 마우스 캡처 토글 | 끄면 마우스를 터미널에 넘겨 네이티브 클릭-드래그 선택과 우클릭 컨텍스트 메뉴를 복원하지만, 앱 내 드래그 선택, 트랜스크립트 스크롤바, 휠 스크롤은 희생됩니다. `PATTY_DISABLE_MOUSE=1`을 설정하면 모든 세션을 끈 상태로 시작합니다. |
| `Ctrl+C` | 복사, 취소, 지우기 또는 종료 | 활성 트랜스크립트 또는 컴포저 선택 영역을 먼저 복사합니다. 그 외에는 실행 중인 턴을 취소하고, 비어 있지 않은 입력을 지우거나, 빈 컴포저에서 두 번째 누름에 종료합니다. |
| `Ctrl+D` | TUI 종료 | 즉시 종료합니다. |
| 터미널의 텍스트 붙여넣기 단축키 | 텍스트 붙여넣기 | 텍스트는 터미널의 bracketed-paste 경로에 유지됩니다(macOS의 `Cmd+V`, Linux에서 흔히 `Ctrl+Shift+V`, 그 외에는 터미널에 구성된 단축키). Patty Code는 결과 붙여넣기 이벤트를 소비하며 이미지를 먼저 검사하지 않습니다. |
| macOS/Linux에서 `Ctrl+V`; Windows에서 `Alt+V` | 클립보드 이미지 붙여넣기 | 이미지 붙여넣기는 별도의 애플리케이션 동작입니다. 클립보드를 읽는 동안 푸터에 `Pasting image…`가 표시된 후 커서 위치에 편집 가능한 `[image #N]` 토큰을 삽입합니다. |
| `/paste-image` | 클립보드 이미지 붙여넣기 | 동일한 이미지 전용 동작의 명령 형태입니다. |
| `!`로 시작하는 줄 | 셸 명령 직접 실행 | 모델에게 묻지 않고 로컬에서 명령을 실행합니다. |

모드 및 표시 단축키:

| 키 또는 명령 | 기능 | 참고 |
| --- | --- | --- |
| `Shift+Tab` | Ask → Auto → Plan → Ask 순환 | YOLO는 이 컴포저 모드 순환 밖에 있으며, 푸터에 활성 모드가 표시됩니다. |
| `Ctrl+Y` | YOLO 켜기/끄기 | YOLO를 끄면 알려진 경우 이전 Ask/Auto 기본값을 복원합니다. Command/Super를 전달하는 터미널은 `Cmd+Y`도 보낼 수 있지만, 신뢰할 수 있는 터미널 단축키는 `Ctrl+Y`입니다. |
| `--yolo`, `--dangerously-skip-permissions` | YOLO로 채팅 시작 | `Ctrl+Y`와 동일한 런타임 모드입니다. |
| `/work-mode [economy|balanced|delivery]` | 현재 세션의 작업 모드 표시 또는 전환 | `/profile`은 호환성 별칭입니다. 전환은 런타임을 원자적으로 재구축하고 대화와 승인 자세를 보존하며, 작업이 진행 중이면 차단됩니다. |
| `/theme [auto|light|dark|style]` | CLI 테마 표시 또는 전환 | `/theme`만 입력하면 배경 모드와 명명된 강조 팔레트가 나열됩니다. 선택은 사용자 구성에 저장되며, `PATTY_THEME`와 `PATTY_THEME_STYLE`로 한 번의 실행을 재정의할 수 있습니다. |
| `Ctrl+O` | 상세 reasoning 표시 토글 | `/verbose`로도 사용할 수 있습니다. |
| `Ctrl+B` | 긴 셸 출력 펼치기/접기 | 긴 셸 출력 힌트 줄은 트랜스크립트에서 클릭할 수도 있으며, 전체 화면 TUI가 마우스 보고를 활성화한 동안 텍스트 선택은 앱 내에서 처리됩니다. |
| `/goal <objective>`, `/goal --research <objective>`, `/goal --simple <objective>`, `/goal status`, `/goal clear` | Goal 시작, 확인 또는 지우기 | Goal은 어떤 키보드 순환에도 없습니다. 명확히 장기적인 목표는 Goal이 명시적으로 시작된 후 AutoResearch를 자동으로 활성화합니다. |
| `/migrate`, `/migrate --from <legacy-dir>` | 레거시 마이그레이션 재시도 또는 선택한 v0.x 소스에서 세션 가져오기 | 사용자 지정 Windows v0.52 설치/데이터 디렉터리에는 `--from`을 사용하세요. 세션만 가져옵니다. [구성 경로](./CONFIG_PATHS.md)를 참조하세요. |

선택기 및 승인 단축키:

| 컨텍스트 | 키 | 기능 |
| --- | --- | --- |
| 슬래시 또는 `@` 완성 | `Up` / `Down`, `Ctrl+P` / `Ctrl+N`, `Tab` / `Enter`, `Esc` | 완성 메뉴 이동, 수락 또는 닫기. |
| 도구 승인 프롬프트 | `y`/`1`, `a`/`2`, `p`/`3`, `n`/`4`, `Enter`, `Esc`, `Ctrl+C` | Allow once, allow for session, persist allow, deny, 기본 Allow once 수락, 거부 또는 턴 취소. |
| Ask 질문 카드 | `Up`/`Down` 또는 `j`/`k`, `Left`/`Right` 또는 `h`/`l`, `Space`, `Enter`, `1`-`9`, `Esc`, `Ctrl+C` | 답변/탭 탐색, 다중 선택 답변 토글, 제출/활성화, 번호 옵션 선택, 닫기 또는 턴 취소. |
| 되감기 선택기 | `Up`/`Down` 또는 `j`/`k`, `Enter`, `b`, `c`, `d`, `f`, `s`, `u`, `Esc` | 턴 선택, both/conversation/code/fork/summarize 동작 적용 또는 돌아가기/닫기. |
| 모델, 제공자 또는 재개 선택기 | `Up`/`Down` 또는 `Ctrl+P`/`Ctrl+N`; 검색이 비어 있으면 `j`/`k`; 입력하여 필터; `Enter`; `Esc` | 검색, 항목 선택 또는 선택기 닫기. 검색 입력이 시작되면 `j`/`k`는 쿼리 텍스트가 됩니다. `/provider`는 해당 제공자의 모델 목록을 엽니다. |
| MCP 가져오기 선택기 | `Up`/`Down` 또는 `j`/`k`, `Space`, `Enter`, `Esc` / `Ctrl+C` | 이동, 서버 선택, 선택한 서버 가져오기 또는 취소. |
| MCP 관리자 | `Up`/`Down` 또는 `j`/`k`, `Enter`, `Left`/`Right` 또는 `h`/`l`, `r`, 숫자 키, `q` / `Ctrl+C` | 서버 목록/세부 정보 탐색, 새로고침, 동작 선택 또는 닫기. |
| `/clear` 확인 | 화살표 키 또는 `j`/`k` / `Tab`, `Enter`, `y`, `n`, `Esc` / `Ctrl+C` | Clear/Cancel 토글, 지우기 확인 또는 취소. |

모드 의미:

| 모드 | 의미 |
| --- | --- |
| Ask | 폴백 작성자 승인을 프롬프트합니다. |
| Auto | 폴백 승인을 자동 허용하며, 명시적 `ask` / `deny` 규칙은 계속 적용됩니다. |
| YOLO | 일반 도구 승인 프롬프트를 건너뜁니다. `deny`, 사용자 `ask` 질문, 플랜 승인 프롬프트는 계속 기다립니다. |
| Plan | 모델이 먼저 계획하도록 지시합니다 — 모든 도구 읽기 전용 모드가 아닌 계획 우선 워크플로입니다. 내장 작성자는 활성 Ask/Auto/YOLO 규칙과 Sandbox를 계속 따르며, 설치된 MCP 작성자, 파괴적 대상, 승인되지 않은 서버의 읽기 도구는 전체 계획 단계 동안 하드 차단됩니다(승인이 해제할 수 없으며 Plan이 종료되면 복귀합니다). `complete_step` 같은 명시적 단계 전용 도구는 승인까지 기다립니다. |
| Goal | 완료, 차단 또는 지우기까지 저장된 목표를 추구합니다. |

## 권한 및 샌드박스

Permissions는 각 도구 호출을 게이트합니다: `deny` > `ask` > `allow` > 폴백. Bash와 파일 변경 도구는 기본적으로 승인이 필요하며, 읽기 전용 도구는 일반적으로 필요하지 않습니다. 승인은 버튼 레이블이 아닌 권한 규칙으로 저장되고 일치합니다. 예: `Bash(npm run build)`, `Bash(npm run test:*)`, `Edit(docs/**)`. `patty`는 Bash를 정확한 명령 또는 보수적인 명령 접두사(예: `Bash(go test:*)`)로 부여할 수 있으며, 파일 편집 도구는 세션 편집 허가를 공유하고 `Edit(src/app.go)` 같은 경로 범위 규칙을 유지합니다. 매개변수/산술 확장, 할당, 히어독, 파일 리다이렉트, 글롭은 단순한 Bash, 접두사 또는 글롭 허용을 재사용할 수 없습니다. 사용자가 승인한 재사용 가능한 선택은 전체 명령을 `Bash=<literal>`로 저장합니다. 이들은 여전히 일반 폴백을 따르므로 Auto는 추가 프롬프트 없이 실행합니다. 명령/프로세스 치환, 동적 명령 이름, `eval`, `source`, 셸 `-c`, 인라인 런타임 코드, 파싱할 수 없는 형태는 대화형 Ask/Auto에서 사람을 요구합니다. 헤들리스 Ask/Auto/DontAsk는 정확한 리터럴이 존재하지 않는 한 이 중첩/간접 클래스를 거부하며, YOLO는 이를 우회할 수 있습니다. 고급 사용자는 `[permissions] allow_dynamic_bash = true`를 설정하여 Allow 폴백(Auto 포함)이 해당 클래스를 다루게 할 수 있습니다. 명시적 `ask` 및 `deny` 규칙은 계속 우선합니다.
헤들리스 실행에는 승인 UI가 없으므로 기본 Ask 자세도 일반 작성자 폴백과 명시적 ask 규칙에서 닫힘(fail closed)으로 실패합니다. 무인 자동화가 일반 작성자 폴백을 허용해야 하면 `patcode run --auto ...`, `-y` 또는 `--permission-mode auto`를 사용하세요. 구성된 `ask` 및 `deny` 규칙은 항상 권위를 유지합니다.

Ask는 읽기 전용이 아닙니다: 승인 후에도 작성자는 계속 실행할 수 있습니다. Permissions는 허용할지 프롬프트할지를 결정하고, Sandbox는 강제되는 기능 경계입니다. 샌드박스는 인가 후의 두 번째 경계로 남으며, 격리만으로는 모호한 명령 파싱을 자동 승인에 안전하게 만들 수 없습니다.

Permissions는 *정책*(어떤 호출을 허용/프롬프트할지)입니다. **샌드박스**는 *집행*입니다: 파일 작성자(`write_file` / `edit_file` / `multi_edit` / `move_file`)는 `[sandbox] workspace_root`(기본값: 현재 디렉터리, 따라서 편집이 프로젝트에 유지됨) 밖의 모든 경로를 거부하며, 심볼릭 링크와 `..`를 해석하여 링크가 바깥으로 빠져나가지 못하게 합니다. `forbid_read`는 선택적으로 민감한 파일이나 디렉터리를 에이전트의 읽기/목록/검색 도구에서 숨깁니다. 구성 확장이 환경 변수 기반이므로 `~`가 아닌 절대 경로 또는 `${HOME}` / `${VAR}` 참조를 사용하세요. `bash`는 OS 샌드박스를 사용할 수 있을 때 기본적으로 자체적으로 감옥에 갇힙니다(`[sandbox] bash`, macOS의 Seatbelt, Linux의 bubblewrap): 명령은 동일한 루트와 플랫폼별 명령 임시/캐시 루트에만 쓸 수 있고, OS 샌드박스가 활성화된 동안 구성된 `forbid_read` 루트를 읽을 수 없으며, `[sandbox] network`가 설정된 경우에만 네트워크에 도달합니다. Patty Code는 항상 저장된 제공자 및 봇 자격 증명 변수를 도구 하위 프로세스 환경에서 제거하고 전역 자격 증명 `.env`를 런타임 읽기 거부 경계에 자동으로 추가합니다. 프로젝트 `.env` 파일은 기존 워크스페이스 범위 동작을 유지합니다.

**세션 전용 임시 디렉터리.** 하나의 논리적 채팅 세션 내에서 Bash 명령은 전용 임시 디렉터리를 공유하므로 연속 호출이 `$TMPDIR`을 통해 파일을 교환할 수 있습니다(Linux의 bubblewrap에서는 리터럴 `/tmp`를 통해서도). 사용자 설정이 필요 없습니다: Patty Code는 Bash 및 클라이언트 소유 ACP 터미널에 `TMPDIR`, `TMP`, `TEMP`를 자동으로 내보냅니다. 디렉터리는 지연 생성되며 호스트 공개 임시 루트가 아니고, `/new`, `/clear`, 다른 세션 재개, 브랜치 전환 시 순환됩니다. 모델/설정 핫 재구축은 동일한 디렉터리를 유지합니다. 임시 파일은 내구성 있는 저장소가 아닙니다: 프로세스 재시작 간 재개는 이를 복원하지 않으며, 오래 지속되는 데이터가 필요한 스크립트는 워크스페이스나 사용자 지정 경로에 써야 합니다.

Patty Code 생성 및 프로젝트 스크립트는 `/tmp`를 하드코딩하지 말고 표준 임시 환경 변수를 사용해야 하며, 사용자가 이 변수를 직접 설정해서는 안 됩니다. 예:

```sh
tmp_file="${TMPDIR:?}/result.json"
```

```powershell
$tmpFile = Join-Path $env:TEMP "result.json"
```

| 플랫폼 | `$TMPDIR` / `$TMP` / `$TEMP` | 리터럴 `/tmp` |
| --- | --- | --- |
| Linux + bubblewrap | 가상 `/tmp`(전용 디렉터리에 바인딩) | 세션에 공유(호출마다 새 빈 tmpfs가 아님) |
| macOS Seatbelt | 전용 디렉터리의 호스트 경로(정책으로 허용) | 호스트 macOS 임시 디렉터리; 스크립트는 `$TMPDIR` 사용 |
| Windows(OS Bash 샌드박스 없음) | 전용 디렉터리의 호스트 경로 | 일치가 보장되지 않음(예: Git Bash `/tmp`) |

MCP 서버 같은 독립 샌드박스는 자체 격리를 유지하며 채팅 세션의 임시 디렉터리를 상속하지 않습니다. 승인된 샌드박스 탈출 명령도 전용 임시 환경 변수를 받지만, Linux에서는 리터럴 `/tmp`가 더 이상 bubblewrap으로 매핑되지 않습니다.

**Windows 참고:** Patty Code는 Windows에서 OS 수준 Bash 샌드박스를 제공하지 않습니다. 유효 모드는 `off`로 고정되며, `bash = "enforce"`가 포함된 이전 구성도 `off`로 해석되고, `patcode doctor`가 무시된 값을 표시하며, 데스크톱 선택기는 읽기 전용입니다. 따라서 Bash 명령은 제한 없이 실행되지만, 전용 파일 도구는 프로세스 내에서 `workspace_root`, `allow_write`, `forbid_read`를 계속 강제합니다. 저장된 자격 증명 변수는 여전히 하위 환경에서 제거되지만, 승인된 비제한 셸은 사용자로 실행되며 다른 사용자 읽기 가능 파일에 대한 보안 경계가 아닙니다.

OS 샌드박스 백엔드를 사용할 수 없을 때 `bash = "enforce"`는 비제한 실행 대신 bash 실행을 거부합니다. 플랫폼 샌드박스 백엔드(Linux의 bubblewrap/`bwrap`, macOS의 `sandbox-exec`)를 설치하거나 `[sandbox] bash = "off"`를 설정하여 1.16 이전의 비제한 셸 동작을 명시적으로 복원하세요. Windows에서 호환 값은 항상 `off`입니다.

코딩 품질 리포트는 `patcode doctor quality <branch-id-or-path>`를 실행하세요(`--json`을 추가하면 구조화된 출력). 이 명령은 선택한 세션을 읽지만 콘텐츠 없는 카운트와 프로필 범주만 출력합니다: 모델 계열, 런타임 프로필, 협업/승인 모드, 메시지 및 도구 호출 수, 검증 및 유지된 압축 요약 카운트, 사용 가능한 데스크톱 토큰/캐시 텔레메트리. 트랜스크립트 텍스트, 경로, 세션 식별자, 도구 인수와 출력, 엔드포인트, 사용자 지정 모델 이름을 생략하므로 공개 이슈 또는 Discussion에 적합합니다. 이는 완전한 비가림 트랜스크립트가 포함된 지원 zip을 만드는 `patcode doctor session`과 다릅니다. 후자는 신뢰할 수 있는 지원 채널에 남아 있어야 합니다.

## 기능 진단

스킬, 슬래시 명령, 훅, 플러그인 패키지, MCP 서버 또는 `AGENTS.md`가 누락되었거나, 가려졌거나, 비활성화되었거나, 시작에 실패할 때 사용하세요. 전체 플래그 참조, JSON 스키마, 이슈 코드: **[기능 진단](../reference/CAPABILITY_DIAGNOSTICS.md)**.

```bash
# 정적(기본값): 네트워크 없음, MCP 하위 프로세스 없음
patcode doctor capabilities

# 기계 판독 가능(stdout은 순수 JSON)
patcode doctor capabilities --json

# 다른 워크스페이스 루트
patcode doctor capabilities --root /path/to/project

# 라이브 MCP 프로브 — 타사 서버 시작을 명시적으로 허용할 때만
patcode doctor capabilities --live --timeout 5s
```

| 표면 | 방법 |
| --- | --- |
| CLI | `patcode doctor capabilities`(위) |
| Desktop | **Settings → Diagnostics** — 새로고침, 가림 처리된 JSON 복사, 선택적 "include current session runtime"(활성 탭 Host만 읽음; MCP를 시작하지 **않음**) |
| Agent | `/patty-guide`(내장 인라인 스킬) 또는 자연어로 질문; `--live`보다 정적 doctor JSON을 우선함 |

종료 코드 `0`은 경고/정보를 허용하고, `1`은 `error`가 하나 이상(또는 라이브 시작 실패)임을, `2`는 잘못된 플래그를 의미합니다. 이는 `patcode doctor`(제공자/샌드박스) 및 `patcode plugin doctor <name>`(단일 패키지)와 별개입니다.

## 플러그인(MCP)

Patty Code는 MCP 클라이언트입니다. `[[plugins]]` 항목의 `type`이 전송 방식을 선택합니다: `stdio`(기본값)는 로컬 하위 프로세스를 실행하고(`command`/`args`/`env`), `http`(Streamable HTTP)는 선택적 정적 `headers`와 함께 원격 `url`에 연결하며(`${VAR}` / `${VAR:-default}`는 환경에서 확장되므로 토큰이 파일에 남지 않음), `sse`는 레거시 지속 GET + 공지된 POST 엔드포인트 전송을 여전히 사용하는 서버에 연결합니다.

공식 MCP Registry는 **Settings → MCP servers → Browse registry**에서 탐색하거나 `patcode mcp browse [query]` 및 `patcode mcp install <registry-name>`을 사용하세요. 레지스트리 접근은 명시적이며 시작 중에는 절대 실행되지 않습니다. 비밀값이나 필수 인수가 필요한 항목은 불완전한 구성으로 설치되는 대신 수동 설정으로 표시되며, 레지스트리 중단 중에도 쿼리별 캐시된 결과는 계속 사용할 수 있습니다.

일반 설정 경로는 의도적으로 한 단계입니다. Desktop의 **Add and connect**, `/mcp add`를 사용하거나 Patty Code에 패키지나 URL 설치를 요청하세요. 이러한 명시적 설치는 사용자 전역 `config.toml`에 저장되며 인가이기도 합니다: 서버는 현재 세션에서 연결되고, 지금이나 다음 시작 시 두 번째 신뢰 단계가 나타나지 않습니다. 현재 프로젝트의 `patty.toml` 또는 `.mcp.json`이 선언한 서버는 해당 프로젝트에 남으며 별도의 실행 확인 없이 신뢰됩니다. 명시적 deny 규칙은 여전히 우선합니다. 서버의 호출은 `destructiveHint`를 선언하는 도구를 포함해 직접 실행됩니다. 전용 Planner는 여전히 파괴적 도구를 거부하고, 엄격한 읽기 전용 하위 에이전트는 여전히 힌트된 비파괴적 읽기 도구만 노출합니다.

MCP 이름은 워크스페이스당 한 번 해석됩니다. 프로젝트 선언은 동일한 이름의 전역 설치를 재정의합니다. 프로젝트 내에서 `patty.toml`이 `.mcp.json`을 재정의합니다. 편집은 원래 파일의 유효 선언을 업데이트하며, 더 높은 우선순위 선언을 제거하면 모든 동일 이름 항목을 삭제하는 대신 다음 항목이 드러납니다.

stdio 서버는 initialize, 읽기, 쓰기에 하나의 프로세스를 유지하므로 브라우저 같은 상태 저장 서버는 세션과 열린 페이지를 보존합니다. OS 샌드박스는 프로세스 시작 시 고정되므로 이 공유 프로세스는 모든 호출에 서버의 일반 프로세스 샌드박스를 사용합니다. `readOnlyHint` 및 읽기 전용 하위 에이전트 필터링은 호출별 두 번째 프로세스 샌드박스가 아닌 디스패치 정책입니다.

도구는 `mcp__<server>__<tool>`로 모델에 표면화됩니다. MCP의 `readOnlyHint: true`를 선언하는 도구는 병렬 디스패치와 엄격한 읽기 전용 도구 표면에 합류합니다. 서버를 설치하거나 프로젝트 구성에 선언하면 전용 Planner가 추가 도구별 설정 없이 해당 비파괴적 도구를 모두 사용할 수 있게 인가하며, 엄격한 읽기 전용 연구 하위 에이전트는 힌트된 비파괴적 읽기 도구만 받습니다. 힌트가 없는 도구는 스케줄링 및 변경 회계를 위해 쓰기 가능 상태로 남습니다. 계획 중에 내장 작성자는 일반 권한 자세를 유지합니다. 전용 Planner는 인가된 비파괴적 MCP(불투명 작성자 포함)를 허용하지만 파괴적 또는 인가되지 않은 대상을 하드 차단합니다. 전용 Planner가 없는 단일 모델 Plan은 Plan이 종료될 때까지 기존의 작성자/파괴적 차단을 유지합니다.

MCP 서버 설치는 인가 결정입니다. 설치 후 모든 도구는 두 번째 서버 수준, 도구별, 작성자 또는 파괴적 승인 설정 없이 직접 실행됩니다. 명시적 전역 deny 규칙은 여전히 우선합니다. 호스트는 `readOnlyHint`와 `destructiveHint`를 병렬 스케줄링, Plan 제한, 엄격한 읽기 전용 하위 에이전트, 캐시-라이브 안전 재분류를 위해 내부적으로 유지합니다. 이 힌트는 사용자 구성을 추가하지 않습니다.
Patty Code는 설치된 서버가 해당 힌트를 정직하게 설명한다고 의도적으로 신뢰합니다. 따라서 Planner/읽기 전용 필터링은 악성 MCP 서버에 대한 격리가 아니라 신뢰된 서버를 위한 워크플로 경계이며, 명시적 deny 규칙과 프로세스 샌드박스는 호스트 제어 경계로 남습니다.

폐지된 `trusted_read_only_tools`, `default_tools_approval_mode`, `tools.<raw>.approval_mode`, `approvals_reviewer` 필드는 이전 파일을 로드할 때 무시되며, Patty Code가 해당 MCP 항목을 다음에 저장할 때 제거됩니다.

서버의 **prompts**는 `/mcp__<server>__<prompt>` 슬래시 명령으로 표면화되고(명령 뒤에 위치 인수), **resources**는 메시지에 `@<server>:<uri>`를 써서 가져옵니다. `/mcp`는 연결된 서버와 각 서버가 노출하는 것을 나열합니다. `make build`는 또한 복사할 수 있는 실행 가능한 참조 stdio 서버(`echo`, `wordcount`, `review` 프롬프트, 스타일 가이드 리소스)인 `bin/patty-plugin-example`을 생성합니다.

```toml
[[plugins]]                       # 로컬 stdio 서버
name    = "example"
command = "patty-plugin-example"
# startup_timeout_seconds = 60    # 선택적 initialize + tools/list 상한
# call_timeout_seconds = 600       # 선택적 서버별 MCP 호출 타임아웃
# tool_timeout_seconds = { "generate_video" = 1800 }   # 선택적 원시 MCP 도구 이름

[[plugins]]                       # Streamable HTTP를 통한 원격 서버
name    = "stripe"
type    = "http"
url     = "https://mcp.stripe.com"
headers = { Authorization = "Bearer ${STRIPE_KEY}" }
```

활성화된 MCP 서버는 세션 시작 후 백그라운드에서 자동으로 연결을 시작하므로 도구가 온라인이 되는 동안 채팅을 계속 사용할 수 있습니다. `/mcp` 또는 데스크톱 MCP 패널로 상태를 새로고침하고, 서버를 재연결하고, 실패를 검사하거나 현재 세션에서 서버를 비활성화하세요. 스킬, 훅, 패키지, MCP에 걸친 읽기 전용 구성/런타임 상태 보고서(설정 변경 없음)는 [기능 진단](../reference/CAPABILITY_DIAGNOSTICS.md)(`patcode doctor capabilities` 또는 **Settings → Diagnostics**)를 참조하세요.

대화형 호출자는 콜드 서버를 잠시만 기다립니다. 그 대기가 끝나면 공유 시작은 종료되고 재시작되는 대신 백그라운드에서 계속됩니다. 도구가 온라인이 된 후 재시도하세요. `mcp_startup_timeout_seconds`(기본값 `30`)는 전체 시작, 인가, initialize, `tools/list` 시퀀스를 제한합니다. `mcp_call_timeout_seconds`는 서버 연결 후에만 적용됩니다. 두 값 모두 서버별로 재정의할 수 있습니다.

**이미 `.mcp.json`이 있나요?** 프로젝트 루트에 놓으면 Patty Code가 그대로 읽습니다 — `mcpServers` 스펙(`command`/`args`/`env`, `type`/`url`/`headers`, `${VAR}` 확장)은 `[[plugins]]`에 필드 대 필드로 매핑됩니다. 두 소스가 병합되며, 이름 충돌 시 `patty.toml`이 우선합니다.

```json
{
  "mcpServers": {
    "filesystem": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"] },
    "stripe": { "type": "http", "url": "https://mcp.stripe.com", "headers": { "Authorization": "Bearer ${STRIPE_KEY}" } }
  }
}
```

**`0.x`에서 업그레이드 중인가요?** 이전 `~/.patty/config.json`의 `mcpServers`(`mcpDisabled` 존중)는 가장 낮은 우선순위 소스로 계속 읽히므로 MCP 서버가 계속 작동합니다. 편할 때 `patty.toml`의 `[[plugins]]` 또는 `.mcp.json`으로 옮기세요.

## 슬래시 명령

대화형 `patty` 세션에서 내장 명령(`/compact`, `/new`, `/clear`, `/rewind`, `/tree`, `/branch`, `/switch`, `/todo`, `/model`, `/work-mode`, `/mcp`, `/skills`, `/hooks`, `/memory`, `/goal`, `/output-style`, `/sandbox`, `/language`, `/reasoning-language`, `/help`)은 로컬에서 실행됩니다 — `/help`가 모두 나열합니다. `/init`, `/explore`, `/test`, `/patty-guide` 같은 내장 **스킬**도 슬래시 메뉴와 `run_skill`로 나타납니다(본문은 요청 시 로드되며 인덱스 줄만 캐시 안정적). 구성 또는 기능 문제 해결이 필요하면 `/patty-guide`를 사용하세요. 이는 `patcode doctor capabilities`를 가리킵니다([기능 진단](../reference/CAPABILITY_DIAGNOSTICS.md) 참조). `/new`는 이전 트랜스크립트를 기록/재개용으로 저장하면서 새 세션을 시작하고, `/clear`는 확인을 요청한 후 현재 컨텍스트를 저장하지 않고 버립니다. `/tree`는 저장된 대화 브랜치를 표시하고, `/branch [name]`은 현재 대화 팁에서 포크하며, `/branch <turn> [name]`은 이전 체크포인트 턴에서 포크하고, `/switch <id|name>`은 다른 브랜치를 로드합니다. **사용자 지정 명령**은 `.patty/commands/`(프로젝트) 또는 `~/.patty/commands/`(사용자) 아래의 Markdown 파일입니다 — `review.md`는 `/review`가 되고, 하위 디렉터리는 네임스페이스를 제공합니다(`git/commit.md` → `/git:commit`). 본문은 프롬프트 템플릿이며, 명령을 호출하면 턴으로 전송됩니다.

### 하위 에이전트 프로필

하위 에이전트 프로필은 `runAs: subagent`와 `invocation: manual`이 있는 수동 Skills입니다. 데스크톱 설정 페이지와 동일한 프로젝트/전역 Skill 루트에 저장되므로 어느 화면에서 만든 프로필도 세션 새로고침 후 즉시 다른 화면에서 사용할 수 있습니다. 대화형 채팅에서 `/<name> <task>`로 호출하면 Patty Code가 격리된 하위 루프를 실행하고 부모 대화에는 작업과 최종 답변만 유지합니다.

헤들리스 CLI는 일반 `patcode run` 작업 의미를 변경하지 않고 명시적 관리 및 실행 명령을 제공합니다:

```bash
patcode subagent list
patcode subagent create reviewer --description "Review changes" --prompt-file reviewer.md --tools read_file,grep,bash
patcode subagent edit reviewer --model patty/medium
patcode subagent try reviewer "review the current diff"   # 항상 읽기 전용
patcode subagent run reviewer "review and fix the current diff"
patcode subagent delete reviewer --yes
```

`create`는 워크스페이스를 사용할 수 있으면 프로젝트 범위를, 그렇지 않으면 전역 범위를 기본값으로 합니다. 명시적으로 선택하려면 `--scope project|global`을 전달하세요. `edit`는 명시적으로 제공된 필드만 변경하며, `--model=` 또는 `--tools=` 같은 빈 값은 해당 필드를 지웁니다. 프로필 편집기는 사용자 지정 경로나 더 풍부한 수제 Skills를 의도적으로 거부하여 frontmatter, 참조 또는 스크립트가 버려지지 않게 합니다. 해당 파일은 Skills 워크플로로 관리하세요. 내장 프로필에는 편집 가능한 파일이 없으므로 `edit`는 이들에 대해 `--model`과 `--effort`만 허용하고 데스크톱 설정 페이지와 동일한 이름별 재정의를 저장합니다.

전체 CLI 참조, Skill 파일 형식, 모델 우선순위, 안전 동작, 문제 해결은 [하위 에이전트 프로필](../reference/SUBAGENT_PROFILES.md)을 참조하세요.

Context Engine v2는 의도적으로 다른 두 계층을 분리합니다:

- **상시 지침(Standing instructions)** 은 계층적 `PATTY.md`, `AGENTS.md`, `CLAUDE.md` 파일에서 옵니다. 관련된 모든 턴에 존재해야 하는 규칙을 여기에 두세요. 사용자 전역 파일이 먼저 로드된 다음 워크스페이스 및 더 깊은 대상 디렉터리가 로드되며, 한 디렉터리 안에서는 `.local.md` 변형이 우선합니다.
- **배경 메모리(Background memory)** 는 Markdown 파일당 하나의 내구성 있는 사실을 저장합니다. 각 사실은 불변 ID, 단조 개정 번호, 타임스탬프, 독립적인 `type`(`user`, `feedback`, `project`, `reference`)과 `scope`(`project`, `global`), 신선도 메타데이터를 가집니다. 사실은 오래되었을 수 있으므로 현재 요청이나 상시 지침보다 우선하지 않습니다.

Patty Code는 각 실제 사용자 턴 전에 관련 사실의 작은 집합을 자동으로 회상합니다. 원시 사용자 메시지를 검색하고 "continue" 같은 일반 요청을 억제하며, 동등한 전역 폴백보다 프로젝트 사실을 선호하고, 오래된 사실을 낮은 순위로 두며, 사용자 턴에 최대 4개의 사실 / 2,400자를 추가합니다. 이 동적 접미사는 캐시 안정적 시스템 프롬프트나 도구 스키마를 다시 쓰지 않습니다. 선택된 ID, 점수, 사유, 신선도, 예산, 억제 결정을 보려면 `/memory recall`을 사용하세요.

새롭고 제한적이며 민감하지 않은 프로젝트/참조 사실은 설정이나 승인 클릭 없이 자동으로 생성할 수 있습니다. 전역 사실, 사용자 기본 설정, 피드백, 업데이트, 중복, 민감/과대 콘텐츠, 모든 `forget`은 여전히 명시적 확인이 필요합니다. 저장 계층은 자동 허가를 생성 전용으로 만들어 동시에 나타나는 사실을 덮어쓸 수 없게 합니다. 최상위 헤들리스 컨트롤러는 동일한 일회성 저위험 생성 경로를 사용할 수 있으며, 소유 범위 컨트롤러가 없는 하위 에이전트와 헤들리스 표면은 닫힘으로 실패합니다.

`forget`는 영구 삭제가 아니라 보관입니다. 모든 업데이트는 이전 개정을 스냅샷하고, 복원 및 보관 복구는 기록을 덮어쓰는 대신 항상 더 높은 개정을 생성합니다:

```text
/memory instructions
/memory recall
/memory revisions <id-or-name>
/memory restore <id-or-name> <revision>
/memory archived
/memory recover <archive-path>
```

데스크톱 Context Center는 동일한 출처, 충돌, 개정 기록, 회상 추적, 복구 동작을 보여줍니다. Suggestions 탭을 열면 최근 로컬 사용자 턴을 자동으로 스캔하며, 후보는 두 메모리 범위와 지침 본문에 대해 중복 제거되지만 사용자가 수락할 때까지 아무것도 저장되지 않습니다. 원격 워크스페이스는 로컬 데스크톱 메모리나 세션으로 절대 폴백하지 않습니다.

레거시 사실은 결정적 ID와 개정 1로 제자리 업그레이드되며, 누락된 범위는 포함 디렉터리에서 추론됩니다. 마이그레이션은 멱등이며, 이전 클라이언트는 안전한 라우팅을 유지하고, 레거시 Memory v5 트랜스크립트는 계속 읽을 수 있습니다. 완전한 동작과 개인정보/캐시 계약은 [`Context Engine v2`](../reference/SESSION_MEMORY_RETRIEVAL.md)를 참조하세요.

```markdown
---
description: Review the staged diff
argument-hint: [focus-area]
---
Review the staged diff. Focus on $ARGUMENTS, list bugs with file:line.
```

`$ARGUMENTS`는 공백으로 구분된 모든 인수로 확장되고, `$1`…`$N`은 위치 인수로 확장됩니다. MCP 프롬프트도 여기에 `/mcp__<server>__<prompt>`로 나타납니다.

## 내장 문서 검색

Patty Code는 `docs/`의 Markdown 파일과 검토된 `release-notes/releases.json` 카탈로그를 각 CLI 및 Desktop 빌드에 번들합니다. 읽기 전용 `docs` 도구는 로컬 BM25 검색으로 그 정확한 오프라인 코퍼스를 검색하고, 소스 출처와 함께 일치하는 완전한 섹션을 읽을 수 있습니다. 모든 릴리스를 `changelog/v1.19.5.md` 및 `changelog/v1.19.5.ko-KR.md` 같은 경로 아래 두 언어로 렌더링하므로 특정 버전, 업그레이드, 수정, 알려진 위험에 대한 질문이 오프라인에서 작동합니다. 에이전트는 질문이 Patty Code 구성, CLI/Desktop 동작, 릴리스 기록, 권한, MCP, 메모리, 복구, 제공자 또는 유지 관리자 워크플로에 관한 것일 때 웹 검색이나 추측보다 먼저 이 도구를 사용해야 합니다.

설정, 네트워크 연결, 벡터 데이터베이스 또는 임베딩 서비스가 필요 없습니다. 검색 결과는 쿼리 언어를 선호하면서 명시적 `en`, `ko-KR`, 대상, 카탈로그 필터를 유지합니다. Balanced 및 Delivery 프로필은 도구를 직접 노출하고, Economy는 요청 시 `docs` 소스를 연결합니다. 모든 결과는 제품 버전, 불변 소스 개정, 코퍼스 SHA-256 다이제스트를 보고합니다. 릴리스 CI는 CLI를 컴파일하고, 내장 매니페스트가 후보의 `docs/*.md`, `release-notes/releases.json`, 빌드 정체성과 일치하지 않으면 게시를 거부합니다. 따라서 최신 온라인 `main-v2` 페이지가 버전 일치 로컬 안내나 릴리스 기록을 조용히 대체할 수 없습니다.

모델을 호출하지 않고 번들 코퍼스 정체성과 사용 예시를 검사하려면 `/docs`를 사용하세요. `/docs <question>`(예: `/docs 1.19.5 changelog`)은 Patty Code가 먼저 로컬에서 코퍼스를 검색한 다음 버전 일치 증거를 현재 구성된 AI에 전달해 출처가 있는 답변을 얻도록 합니다. 이 명령 경로는 모델이 `docs` 도구 호출을 결정하는 데 의존하지 않으며, 일반 자연어 질문은 여전히 도구를 자동으로 사용할 수 있습니다. 기존 사용자 지정 명령과 호환 플러그인 또는 스킬 별칭은 `/docs`의 소유권을 유지하며, 그런 경우 CLI와 Desktop은 보통 내장 코퍼스를 `/patty:docs`로 노출합니다. 그 정규화된 이름도 이미 소유된 경우 Patty Code는 그것을 밀어내지 않고 다음 사용 가능한 `patty:` 한정 폴백을 선택합니다. 원격 Desktop은 호스트의 해석된 명령 카탈로그를 사용하므로 표시된 항목은 항상 해당 호스트가 실행할 것과 일치합니다.

사용자에게 보이는 CLI, Desktop, 구성, 제공자, 권한 또는 도구 동작을 변경하는 풀 리퀘스트는 내장 문서가 업데이트되었는지 선언해야 합니다. 문서 변경이 필요 없을 때는 기존 버전 일치 안내가 올바른 이유를 선언에 설명해야 합니다.

## Goal 및 AutoResearch

Goal은 장기 실행 목표를 위한 통합 런타임입니다. 일반 `/goal` 목표는 가볍게 유지됩니다: Patty Code는 목표가 완료, 차단, 일시 중지 또는 지워질 때까지 계속 작업합니다. 목표가 명확히 장기적이면 Goal은 별도의 `/auto-research` 스킬을 요구하는 대신 AutoResearch 전략을 자동으로 활성화합니다. `auto-research`는 Settings -> Skills나 슬래시 메뉴에 독립형 내장 스킬로 나열되지 않습니다. 일반 채팅은 협업 모드를 암시적으로 변경하지 않습니다. 컴포저에서 Goal을 선택하거나 `/goal`로 장기 실행 목표를 시작하세요.

Goal은 클래스별 **턴** 예산으로 실행됩니다: 단순 목표는 10턴, 쓰기 목표는 20턴, AutoResearch 목표는 40턴이며, 호스트가 검증할 수 있는 진행이 없는 연속 4턴은 목표를 일시 중지합니다. 누적 토큰 사용량은 여전히 추적되어 진단에 표시되지만, **토큰 하드 제한**과 제공자별 요청 허용은 없습니다. Goal 모드에서 단순한 버그/크래시/예외 진술은 사용자가 분석/설명만 요청하거나 변경을 금지하지 않는 한 쓰기 턴 클래스로 기본 설정됩니다. 일시 중지된 목표는 todo, Delivery 체크포인트, 런타임 기록을 유지합니다 — 계속하려면 `/goal resume`(턴 예산 일시 중지는 같은 클래스의 턴을 한 조각 더 추가), 실행 중인 목표를 수동으로 일시 중지하려면 `/goal pause`를 사용하세요. `/goal status`는 전체 런타임 요약(사용/제한 턴, 사용 토큰, 무진행, 확장)을 보여줍니다. 모든 Goal 턴이 끝나면 모델은 구조화된 `update_goal` 도구로 자세(continue/complete/blocked)를 보고하며, 보고가 없으면 독립적인 제한 평가자가 턴을 한 번 판정하고 평가자 실패는 조용히 계속하는 대신 목표를 일시 중지합니다.

복잡한 작업은 목표를 [작업 계약](../reference/TASK_CONTRACT.md)으로 작성하세요: Context, Request, Output format, Constraints, Pause policy. Goal 모드는 이 섹션들을 자율 작업의 경계로 취급합니다. 다음 단계가 되돌릴 수 없거나 외부에서 보이는 작업, 범위 변경 또는 사용자만 제공할 수 있는 정보를 요구하지 않는 한 합리적인 기본값으로 계속 진행합니다.

AutoResearch는 "계속 조사해", "장기 실행", "철저히", "근본 원인이 명확해질 때까지 디버그", "헛돌지 마", "실험 실행", "반복 검증", "이걸 완전한 계획으로 만들어" 같은 강력한 신호가 있는 목표에 활성화됩니다. 목표가 연구/진단, 구현/수정, 검증/테스트, 최적화/문서화/릴리스 같은 여러 단계를 결합하거나 사용자가 기존 `.patty/autoresearch/<task-id>/` 디렉터리를 지정할 때도 트리거될 수 있습니다. 고급 사용자는 `/goal --research <objective>`로 강제하거나 `/goal --simple <objective>`로 경량 Goal을 강제할 수 있습니다. 명시적으로 시작된 Goal 밖에서는 이러한 신호가 일반 채팅 텍스트로 남으며 내구성 있는 AutoResearch 상태를 만들지 않습니다.

AutoResearch가 활성화되면 에이전트는 목표를 채팅 전용 연속이 아닌 상태 저장 연구 루프로 취급합니다. 프로젝트 로컬 `.patty/autoresearch/<task-id>/` 디렉터리를 생성하거나 재사용합니다. 새 작업의 기본 id 형태는 `YYYYMMDD-HHMMSS-slug`이며, 예: `20260618-224530-cache-audit`. Patty Code는 먼저 프로젝트 디렉터리를 확인하고 해당 id가 이미 존재하는 경우에만 `-2`, `-3` 등을 추가합니다. 작업 상태에는 `task_spec.md`, `progress.json`, `findings.jsonl`, `directions_tried.json`, `iteration_log.jsonl`이 포함되며, 각 반복의 방향, 증거, 검증 결과, 차단 요소를 기록하고 `stale_count`로 반복되는 약한 진행을 감지합니다. 반복적인 정체는 증거 소스, 진입점, 테스트 오라클, 분해, 벤치마크 또는 작업자 전략을 바꾸는 구조적 피벗을 강제하며, 같은 전술을 재시도하지 않습니다.

작업자와 하위 에이전트는 독립적으로 탐색할 수 있지만, 오케스트레이터가 정식 상태 파일을 소유합니다. 완료는 `task_spec.md`에 대한 요구사항별 증거 감사가 필요하며, 통과한 좁은 검사가 넓은 요구사항의 증거로 취급되지 않습니다. 동적 실행 상태는 `PATTY.md`, `AGENTS.md`, 프로젝트 메모리, 도구 스키마 또는 캐시 안정적 시스템 프롬프트가 아닌 `.patty/autoresearch/...`에 유지됩니다. 공개 게시, 파괴적 작업, 자격 증명, 결제, 외부 알림은 여전히 일반 승인, 개인정보, 캐시 게이트를 따릅니다.

## @ 참조

메시지에 `@` 참조를 포함하면 Patty Code가 전송 전에 태그된 컨텍스트 블록으로 해석합니다: `@path/to/file`(또는 `@dir`)은 로컬 파일 내용(또는 디렉터리 목록)을 주입하고, `@<server>:<uri>`는 MCP 리소스를 주입합니다. 로컬 경로는 실제로 존재할 때만 참조로 취급되므로 일반 `@멘션`은 리터럴로 유지됩니다. `/` 또는 `@`를 입력하면 자동 완성 메뉴가 열립니다 — 슬래시 명령 또는 계층적 파일 탐색(한 번에 한 디렉터리 수준, 폴더로 하강)과 MCP 리소스.

## 이중 모델 협업

`patcode setup`은 제공자, 모델 목록, 자격 증명, 연결 테스트, 기본 모델을 관리합니다. 변경 사항을 Save and exit까지 스테이징하며 제공자 접근을 데스크톱 앱과 동기화합니다. [CLI 참조](./CLI.md#configure-providers)를 참조하세요. 두 모델을 함께 실행하는 것(executor + planner, 별도의 캐시 안정적 세션)은 이후 한 줄 편집입니다 — `planner_model`을 다른 활성 제공자로 설정하세요:

```toml
[agent]
planner_model = "deepseek/deepseek-v4-pro"   # DeepSeek 추가 후 저빈도 플래너로 사용
```

플래너는 로드된 `PATTY.md` / `AGENTS.md` 메모리와 작은 읽기 전용 연구 도구 세트를 보므로 executor에게 계획을 넘기기 전에 관련 파일을 검사할 수 있습니다. 작성자 및 워크플로 도구는 executor 전용으로 남습니다.

Patty Code는 추가 분류 모델 없이 각 턴을 결정적으로 라우팅합니다: 질문, 짧은 후속 질문, 명확한 원자적 편집, 제한된 읽기 전용 작업은 executor로 직접 가고, 제한된 구현 작업은 짧은 경량 계획을 받을 수 있습니다. 모호하거나, 표면을 넘나들거나, 구조화되었거나, 고위험이거나, 활성 Goal이거나, Delivery 작업은 요청이 명확히 원자적이거나 읽기 전용이 아닌 한 전체 계획을 받습니다. 명시적 Plan Mode는 별도의 호스트 워크플로로 남으며 두 번 계획되지 않습니다. 명시적 `plan first` / `계획 먼저` 요청은 계획을 강제하고, `just do it` / `바로 수정`은 executor로 직접 갑니다. 실행 경계는 요청의 시작뿐 아니라 전체에 걸쳐 인식되며, 인용된 예는 무시됩니다. 단독 plan-first 요청은 플래너에서 executor로 자동으로 계속됩니다. 확인을 기다리라고 명시적으로 말하는 요청은 호스트 승인 경계에서 일시 중지되고 승인 후 executor로 계속됩니다. 명시적 `plan only` / `실행하지 않기` 요청만 계획을 유지한 채 현재 턴을 끝내고 실행하지 않습니다. 이후 사용자 지시가 같은 세션에서 계속될 수 있습니다. 단계 세부 정보는 사용자 프롬프트를 기록하지 않고 진단을 위한 개인정보 안전 경로, 깊이, 사유 코드를 기록합니다.

경량 계획은 간결한 목표, 최대 4개의 정렬된 단계, 가능한 접점, 주요 검증을 포함합니다. 전체 계획은 검증된 접점과 후보 접점을 구분하고 관련 비목표, 위험, 수락 기준, 명령 수준 검증, 되돌리기 어려운 작업의 롤백 지침을 추가합니다. 이러한 계약은 하나의 안정적인 플래너 시스템 프롬프트의 일부이며, 일회성 프롬프트 업그레이드 후 플래너의 접두사 캐시를 보존하기 위해 작은 턴별 깊이 지침만 사용자 턴에 추가됩니다. 호스트는 또한 경량 및 전체 연구에 서로 다른 턴별 라운드 예산을 부여합니다. 플래너가 제한된 연구 및 마무리 라운드 후에도 마무리하지 못하면 일반 계획 및 실행 작업은 원래 작업으로 executor와 함께 계속됩니다. Plan-only 및 승인 게이트 요청은 닫힘으로 실패하며, 불완전한 플래너 턴은 사용할 수 없는 연속 꼬리를 남기는 대신 롤백됩니다.

Patty Code는 일반 실행을 자동으로 관리합니다: 활성 todo가 8개의 도구 호출 라운드 동안 새 완료, 고유 읽기, 명령 또는 변경을 생성하지 않으면 호스트가 executor에게 재평가를 요청합니다. 16개의 무진행 라운드 후에는 다음 사용자 턴에서 재개할 수 있는 저장된 작업과 함께 일시 중지합니다. 정확한 반복은 진행으로 계산되지 않으며, 호스트가 관찰한 새 작업이 리스(lease)를 갱신합니다. 2단계 작업 목록은 동일한 단일 현재 계약을 유지합니다: 활성 레벨 1 하위 단계가 `in_progress` 항목이고 레벨 0 단계는 `pending`으로 유지되며, 하위 단계는 순서대로 작업되고 서명 오프되며 모든 하위 단계가 완료되면 단계 자체가 최종 서명 오프를 위해 `in_progress`가 됩니다.

기존 `[agent].max_steps` 및 `planner_max_steps` 키는 업그레이드 중 구문적으로 계속 허용되지만, 값은 무시되고 일회성 통지와 함께 제거됩니다. 이는 오래된 숨은 제한이 자동 진행이나 상속된 하위 에이전트 작업을 잘라내는 것을 방지합니다. 명시적 실행 예산이 필요하면 일회성 CLI `--max-steps` 플래그를 사용하세요. 무인 봇은 `[bot].max_steps`를 유지합니다.

하위 에이전트 스킬은 기본적으로 executor 모델을 상속합니다. `subagent_model`을 설정하여 다른 구성 모델에서 실행하거나, `subagent_models`로 `review` 또는 `security_review` 같은 특정 스킬만 재정의하세요.

하위 에이전트는 기본적으로 한 계층 더 위임할 수 있습니다: 루트 세션은 깊이 0, 첫 번째 계층 하위 에이전트는 깊이 1이며, 최대 `max_subagent_depth = 2`는 깊이 1 워크플로가 깊이 2 검토자 또는 구현자를 디스패치할 수 있음을 의미합니다. 깊이 2 하위 에이전트는 재귀 에이전트/스킬 도구를 받지 않습니다. `agent.max_subagent_depth = 1`을 설정하면 기존 단일 계층 경계를 복원합니다. 이는 Superpowers 같은 워크플로에서 워크플로 스킬이 검토자 하위 에이전트를 디스패치할 수 있도록 하면서도 무한 재귀와 백그라운드 팬아웃을 피하기 위한 것입니다.

계획에 쓰기 가능한 위임을 부여하지 않고 격리된 더 깊은 연구가 필요하면 `read_only_task`를 사용하세요. 같은 필요를 기존 스킬로 가장 잘 표현할 수 있으면 `read_only_skill`을 사용하세요. 둘 다 읽기 전용 연구 도구와 안전한 포그라운드 bash만 있는 임시 읽기 전용 하위 에이전트를 실행하고, 최종 답변만 반환하며, 재개 가능한 하위 에이전트 트랜스크립트를 만들지 않습니다. 읽기 전용 중첩 위임은 `max_subagent_depth`에 도달할 때까지 사용할 수 있지만, 쓰기 가능한 `task` / `run_skill`은 이 읽기 전용 하위 레지스트리 내부에서 사용할 수 없습니다. 토큰 절약 모드에서 이 격리가 필요하면 `connect_tool_source(source="read_only_skill")`로 이 좁은 표면을 연결하세요. Plan에서 전체 `skills` 소스를 로드하는 것은 허용되며, 이후 작성자 호출은 계속 Permissions/Sandbox를 통과합니다.

모든 엄격한 읽기 전용 하위 항목은 하나의 공유 구성 쌍 — `RunReadOnlySubAgentWithSession` / `NewReadOnlyAgent` — 을 통해 만들어지며, 이는 하위 항목을 영구적으로 읽기 전용으로 표시하고 최종 레지스트리 필터를 적용합니다. 필터는 작성자, 파괴적 MCP 대상, 승인되지 않은 서버의 읽기 도구, 모든 호스트 변형 도구를 제거합니다. 사용자 설치 및 프로젝트 구성 서버는 즉시 인가됩니다. 적격 읽기 도구는 요청 시 시작될 수 있습니다. 다음이 엄격한 읽기 전용 진입점입니다:

| 진입점 | 용도 |
| --- | --- |
| `read_only_task` | 기본 세션에서 격리된 읽기 전용 연구 하위 항목 |
| `parallel_tasks`(읽기 전용) | 동시 읽기 전용 연구 하위 항목 |
| `read_only: true`가 있는 `fleet` | 병렬 프로필 인식 배치(항목별 강제 읽기 전용) |
| `read_only_skill` | 기존 스킬을 구동하는 동일한 격리 |
| `patcode review`(CLI) | diff 또는 브랜치의 읽기 전용 검토 |
| Desktop preview/review 하위 에이전트 | 읽기 전용 데스크톱 분석 표면 |

유지된 세션에서 `parallel_tasks` 및 `fleet`은 모든 전체 답변을 잘림 쉬운 도구 결과로 연결하는 대신 완료된 하위 항목당 제한된 미리보기와 `Subagent reference` 하나를 반환합니다. 부모는 해당 참조로 `read_subagent_result`를 호출하고 `offset_bytes`로 페이지를 넘길 수 있습니다. 결과는 현재 대화 계보와 워크스페이스로 범위가 지정됩니다. 유지된 부모 세션이 없는 헤들리스 실행은 임시로 유지되고 공정한 제한 미리보기를 받지만, 내구성 있는 참조를 만들 수 없습니다.

대화형 이중 모델 Planner는 전용 구성 경로(`NewPlannerAgent`)를 사용합니다: bash, 파일 작성자, 일반 작성자를 계속 차단하지만 `readOnlyHint`를 요구하지 않고 고정 `use_capability` 프록시를 통해 인가된 비파괴적 MCP를 호출할 수 있습니다. 직접 `mcp__*` 스키마는 Planner 도구 목록에 절대 들어가지 않으므로 MCP 설치/연결 변경은 일회성 스키마 업그레이드 후 Planner 캐시 접두사를 변경하지 않습니다. 누락된 `readOnlyHint`는 더 이상 Planner를 차단하지 않으며, `destructiveHint`가 있는 도구는 zero-exec이며 Executor를 위해 계획에 작성되어야 합니다.
Balanced 이중 모델 세션에서 Executor는 동일한 안정적 프록시에 대한 자체 프론트엔드를 가지므로 Planner가 발견한 `auto_start=false` 또는 파괴적 기능은 핸드오프 후에도 기능 ID로 호출 가능합니다. Planner와 Executor의 원장/감사는 격리된 상태로 유지되고 Host 연결만 공유됩니다.

일반 `task` / `fleet` 하위 에이전트도 동일한 고정 프록시(세션 공유 Host 및 연결, 에이전트별 프론트엔드/원장)를 얻고 `readOnlyHint` 없이 설치되거나 프로젝트 구성된 MCP를 호출할 수 있습니다. 해당 호출은 신뢰된 MCP 권한 경로(라이브 인가 + 명시적 deny만)를 사용하며, 작성자 및 파괴적 호출은 여전히 직렬화되고 변경으로 기록되며 Planner 핸드오프가 아닌 Delivery 증거/리스 가드의 적용을 받습니다. 엄격한 `read_only_task` / `read_only_skill` / 검토 하위 에이전트는 안정적 프록시 스키마와 연결 재사용을 공유하지만 엄격한 실행 게이트(`authorized && readOnlyHint && !destructiveHint`)를 유지합니다. 프로필 `allowed-tools` MCP 이름은 프록시에서 기능 ID 허용 목록으로 변환되며, 하위 항목은 동적 `mcp__*` 스키마를 절대 상속하지 않습니다.

엄격한 하위 항목 내부에서 `use_capability`는 커밋/권한/훅/실행 전에 해석된 대상을 다시 확인합니다. 연결되지 않은 적격 MCP 읽기 도구는 현재 스키마 캐시에서 요청 시 시작될 수 있습니다. `tools/call` 전에 캐시된 `readOnlyHint`/`destructiveHint` 사실을 라이브 initialize/tools-list 결과와 대조하며, 읽기-쓰기 변경 또는 파괴적 승격은 zero 실행과 현재 경계를 통한 일반 재시도를 의미합니다. 스키마 전용 변경은 인가된 호출을 중단하지 않고 다음 세션을 위해 캐시를 새로고침합니다. 런타임 활성화, 인가, 완전한 연결 정체성은 디스패치 직전에 다시 확인되므로 다른 프로젝트/탭의 동일 이름 클라이언트가 실수로 재사용될 수 없습니다. 인가되지 않은 서버는 거기서 권한을 높일 수 없습니다. 이 엄격한 하위 경계는 전용 Planner보다 좁습니다: Planner는 인가된 불투명 비파괴적 MCP를 수락하지만, 엄격한 하위 항목은 명시적 읽기 힌트를 요구하고 작성자를 절대 노출하지 않습니다.

시작 런타임 프로필은 `--profile economy|balanced|delivery`로 선택합니다(예: `patcode run --profile delivery "fix and verify this bug"`). Economy는 9개의 도구로 시작합니다: 직접 read/bash/edit/write, 백그라운드 셸 수명 주기 컨트롤, `ask`, `connect_tool_source`. 내장 문서, 전용 검색/파일/워크플로 도구, 세션 기록, 메모리 변경, 슬래시 명령, Skills, MCP, LSP, 웹 접근, 설치, 하위 에이전트는 작업이 필요할 때만 연결됩니다. Balanced는 완전한 도구 표면을 가진 기본값이며, 별도의 Planner가 구성되면 Planner와 Executor 모두 고정 `use_capability` 프록시를 추가합니다. 프록시 스키마는 안정적이지만 Balanced Executor는 의도적으로 직접 `mcp__*` 도구를 유지하므로 해당 직접 도구가 설치, 연결 또는 새로고침될 때 전체 제공자 도구 접두사가 여전히 변경될 수 있습니다. Delivery는 그 완전한 표면을 유지하고, 스키마 변경 없이 요청 시 MCP 검사/호출을 위한 안정적 프록시 도구 하나(`use_capability`)를 추가하며, 수락 기준을 세우고 근본 원인을 수정하고 결과를 검증하고 최종 diff를 검토하기 위한 안정적 계약을 추가합니다. 호스트는 그 계약을 강제합니다: 구체적인 `todo_write` 수락 목록이 존재할 때까지 변경과 검증 명령이 차단되고, 변경된 결과는 검토되고 최신 변경 후 검증되며 `complete_step`으로 서명 오프되기 전에 마무리할 수 없습니다. Skill/MCP `require`/`prefer` 경로는 호스트가 증명한 사유로 호출되거나 거부되어야 하며, 중간/고위험 변경은 구조적 검토(고위험 시 보안 검토)가 필요합니다. `task`, `run_skill`, `review` 같은 메타 도구는 자체로 변경으로 계산되지 않습니다 — 실제 하위 쓰기만 해당합니다. 읽기 전용 분석은 쓰기를 강요하지 않고 계속 사용할 수 있습니다.
대화형 TUI 세션에서 `/work-mode`로 현재 선택을 검사하거나 `/work-mode economy|balanced|delivery`로 전환하세요. `/profile`은 호환성 별칭입니다. 전환은 기록, 세션 경로, 리스, Ask/Auto/YOLO 자세를 보존하면서 컨트롤러를 원자적으로 재구축하며, 턴, 승인/질문, 백그라운드 작업 또는 다른 런타임 전환이 활성화된 동안 거부됩니다. 빌드 실패는 이전 컨트롤러를 사용 가능한 상태로 둡니다. 이 명령은 현재 세션만 변경하고 새 전역 기본값을 유지하지 않습니다. 프로필을 넘으면 새 제공자 캐시 접두사가 하나 생성됩니다. Balanced 및 Delivery 내에서 시스템 계약과 도구 스키마는 안정적으로 유지되며, Economy에서는 각 성공적인 `connect_tool_source` 호출이 연결된 스키마를 다음 요청에 추가하여 도구 표면이 다시 변경될 때까지 안정적인 접두사를 하나 더 만듭니다.

데스크톱 탭은 동일한 세 가지 선택을 노출하고 Economy 또는 Delivery를 유지합니다. 레거시 빈/`full` 값은 Balanced로 유지됩니다.

대화형 프론트엔드에서 Plan Mode는 항상 명시적 사용자 선택입니다. 데스크톱 협업 모드 컨트롤에서 Plan을 선택하거나 CLI에서 `Shift+Tab`으로 Plan으로 순환하세요. Patty Code는 먼저 계획 초안을 작성한 다음 워크플로가 구현으로 전환되기 전에 승인을 기다립니다. 초안 작성 중 이루어진 도구 호출은 여전히 현재 Permissions 및 Sandbox를 사용합니다. 레거시 `agent.auto_plan` 및 `agent.auto_plan_classifier` 값은 업그레이드 중 무시되고 사용자 구성에서 제거됩니다. 표시용 patty code 언어는 세션에서 `/reasoning-language auto|ko-KR|en`으로, 또는 셸/스크립트에서 `patcode config reasoning-language auto|ko-KR|en`으로 변경할 수 있습니다. 의도적으로 프로젝트 로컬 재정의를 원할 때만 reasoning-language 셸 명령에 `--local`을 전달하세요.

별도 세션을 두는 이유(각 모델의 접두사 캐시를 안정적으로 유지)는 [`SPEC.md` §3.5](../reference/SPEC.md#35-two-model-collaboration-coordinator)에 있습니다.
