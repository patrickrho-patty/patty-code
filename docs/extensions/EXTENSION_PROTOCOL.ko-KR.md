# Patty Code Extension Protocol v2

Extension Protocol은 Patty Code(이하 **호스트**)와 프로세스 외부(out-of-process)에서 실행되는 코드 확장(이하 **사이드카**) 사이의 안정적인 유선 계약(wire contract)입니다. `runtime` 블록을 가진 설치된 플러그인이 호스트 바이너리에 링크되지 않고도 런타임 이벤트를 가로채고, 교체 전략을 소유하며, 스트리밍 모델 공급자를 제공하고, 구조화된 UI를 게시하는 방법이 바로 이것입니다.

- 프로토콜 ID: `patty-code.extension.v2`
- 기계 판독 가능 스키마: `internal/extension/protocol/schema.generated.json`
- 메서드/이벤트/제한/오류 색인: `docs/extensions/EXTENSION_PROTOCOL.generated.md` (생성 파일, CI에서 드리프트 검사됨)
- Go SDK(아래 모든 내용 구현): `sdk/go`

이 문서는 생성된 색인에 대한 설명 문서입니다. 내용이 서로 다를 경우 생성된 스키마가 우선합니다.

## Transport

- **NDJSON** 기반의 엄격한 JSON-RPC 2.0: stdin/stdout에 한 줄에 하나의 완전한 JSON 객체를 주고받습니다. stderr는 진단용으로 확장 프로그램에 속하며, 호스트는 오류 처리를 위해 제한된 크기의 자격 증명이 삭제된(redacted) 꼬리 부분만 캡처합니다.
- 프레임은 양방향 모두 **8 MiB**로 제한되며, 크기를 초과하는 프레임은 연결을 종료시키는 `frame_too_large` 오류를 발생시킵니다.
- 요청 ID는 정수입니다. `params`는 객체여야 합니다. 알 수 없는 멤버는 프레임 수준에서 허용되지만 DTO 디코딩은 엄격하므로(알 수 없는 필드는 거부됨) 오타가 즉시 드러납니다.

## Lifecycle

1. 호스트는 사이드카를 생성(exec 형식, 셸 없이)하고 먼저 `extension/initialize`를 보냅니다. params에는 호스트가 수락할 intercepts, replaces, providers, UI 작업 등 매니페스트 기대값이 담깁니다. 하나의 런타임 세대(generation)에 대해 호스트는 공유된 30초 시작 예산 안에서 최대 4개의 사이드카를 병렬로 초기화합니다.
2. 사이드카는 자신의 선언(declaration)으로 응답합니다. 호스트는 이를 검증합니다. 정확한 프로토콜 메이저 버전, 그리고 모든 구독, 교체 슬롯, 공급자, UI 작업은 플러그인 매니페스트의 **부분집합**이어야 합니다. 매니페스트를 벗어나는 모든 것은 `capability_not_declared` 오류로 핸드셰이크에 실패합니다.
3. 호스트는 `extension/initialized`를 보냅니다. 이 시점 이전에 확장 프로그램에서 호스트로 향하는 모든 트래픽은 연결을 손상시킵니다.
4. 종료는 제한적입니다. 타임아웃과 함께 `extension/shutdown`을 보낸 뒤 stdin을 닫고, 사이드카가 종료되지 않으면 프로세스 트리를 강제 종료합니다.
5. 크래시: 죽은 사이드카는 대기 중인 모든 RPC를 취소합니다. 사이드카가 현재 선택된 공급자나 교체 슬롯을 소유하고 있었다면 현재 작업은 명시적으로 실패합니다. 호스트는 절대 다른 모델이나 전략으로 조용히 대체(fallback)하지 않습니다. 크래시된 사이드카는 유휴 시(idle-time) 런타임 리로드에서만 다시 시작됩니다.

## Content references

외부화 가능(externalizable)으로 표시된 페이로드 필드 중 **64 KiB**를 초과하는 것은 호스트 콘텐츠 저장소로 오프로드됩니다. 프레임에는 `ExternalizedField` 설명자(JSON 포인터, 콘텐츠 참조, 바이트 수, SHA-256)와 `null` 자리 표시자가 포함됩니다. 상대편은 `host/content/read`로 **256 KiB** 청크 단위로 바이트를 다시 읽어오며(paging) 바이트 수와 해시를 검증합니다. 단일 콘텐츠 객체는 **8 MiB**로 제한됩니다. 알 수 없거나 만료된 참조는 `content_ref_expired` 오류로 실패합니다.

## Interception

17개의 고정된(frozen) 훅 포인트가 있습니다(생성된 색인 참조). `extension/intercept`는 블로킹 방식이고, `extension/event`는 동일한 포인트에 대한 fire-and-forget 방식의 관찰입니다. 이벤트 전달은 제한된 비차단(non-blocking) 작성자 큐를 사용하며, 큐가 가득 차면 Agent를 멈추게 하는 대신 경고와 함께 관찰을 폐기합니다.

- 일반 인터셉터는 결정적 순서로 **순차적으로** 실행됩니다. 우선순위 오름차순(매니페스트 `priority`, -1000~1000, 기본값 0), 그다음 플러그인 ID, 그다음 등록 순서입니다.
- 호출별 결정: `continue`(페이로드를 그대로 전달), `block`(사용자에게 보이는 사유와 함께 작업 중단), `replace`(페이로드 대체 — 호스트는 사용 전에 해당 포인트의 DTO 및 스키마에 대해 재검증), `allow`/`deny`(`permission.decision`에서만 허용). 전체 신뢰(full-trust) `allow`는 호스트의 거부를 덮어쓰며 감사(audit) 대상입니다.
- 교체 **전략 슬롯**(`system_prompt`, `context`, `provider_request`, `provider_response`, `compaction`, `session_policy`, `permission`, `frontend_events`, `tool:<name>`, `provider:<ref>`)은 설치된 모든 플러그인에 걸쳐 정확히 한 명의 소유자를 갖습니다. 체인(chain)이 먼저 실행되고 슬롯 소유자가 최종 결정권을 가집니다. 전략 소유자의 타임아웃이나 오류는 항상 작업을 실패시킵니다.
- 타임아웃: input/tool/permission 포인트는 기본 5초, session/context/compaction/system-prompt 계열은 30초이며, 매니페스트는 런타임별로 최대 60초까지 조정할 수 있습니다. 타임아웃된 선택적 관찰 전용 확장 프로그램은 한 번 경고 후 건너뛰지만, 필수 확장 프로그램과 슬롯 소유자는 작업을 실패시킵니다.

## Streaming providers

`providers` 기능을 가진 확장 프로그램은 `extension/provider/catalog`에 호스트 공급자와 동등한 설명자(모델, 컨텍스트 창, 가격, 비전, reasoning, effort)로 응답합니다. 자격 증명은 절대 포함되지 않습니다. 모델은 `plugin/<plugin>/<provider>/<model>` 형태로 표시됩니다.

스트림은 `extension/provider/stream/open` → `stream/chunk` → `stream/end` 순서를 따릅니다:

- 청크는 1부터 시작하는 연속 시퀀스 번호를 가집니다. `stream/end.lastSeq`가 종료 경계를 고정합니다. 호스트는 순서가 어긋난 청크를 버퍼링하고 중복을 폐기하며, 빈틈(gap)이 지속되면 누락된 시퀀스를 명시한 채 스트림을 중단(interrupted)으로 실패 처리합니다.
- 청크 유형: `text`, `reasoning`(`signature` 포함), `tool_call_start`, `tool_call_args_delta`, `tool_call`, `usage`(캐시 토큰 포함), `done`, `error`. 공급자 오류는 생성 측에서 삭제(redact)해야 하며 호스트가 방어적으로 한 번 더 삭제합니다.
- 스트림 컨텍스트를 취소하면 `stream/cancel`이 전송되며, 사이드카는 청크 생성을 중단해야 합니다.
- 확장 프로그램은 자신의 환경과 자격 증명을 읽습니다. 호스트는 다른 공급자의 API 키나 헤더를 절대 보내지 않습니다. 크래시된 공급자는 절대 다른 모델로의 대체(fallback)를 유발하지 않습니다.

## Structured UI

`ui` 기능을 가진 확장 프로그램은 `status`, `card`, `form`, `notification` 페이로드를 게시하고(`host/ui/publish`), 질문을 요청합니다(`host/ui/request`: confirm, input, select, multiselect). 서피스(surface)는 **구조화된 것만** 허용됩니다. HTML, CSS, JavaScript, 원격 스크립트, 임의의 프론트엔드 컴포넌트, 제어되지 않는 URL은 없으며, Markdown은 각 프론트엔드의 기존 안전한 렌더러를 통해 렌더링됩니다. 모든 서피스 업데이트에는 플러그인 ID, 서피스 ID, 세션 ID, 런타임 세대가 포함되며, 오래된 세대의 업데이트는 폐기되므로 탭 전환이나 리로드 이후의 늦은 결과가 현재 상태를 덮어쓸 수 없습니다.

initialize 시 선언된 작업은 `/<plugin>:<action>` 네임스페이스로 지정되며 `extension/ui/action`을 통해 호출됩니다. 양식 제출은 `extension/ui/submit`으로 전달됩니다.

## Errors

도메인 오류는 구조화된 데이터(reason, retryable, action)와 함께 JSON-RPC 오류 코드 `-32000`으로 전달됩니다. `protocol_error`, `unknown_method`, `invalid_params`, `internal`은 표준 JSON-RPC 코드를 사용합니다. 고정된 reason 표는 생성된 색인에 있습니다.

## Stability contract

메이저 버전 2 내에서 허용되는 변경은 오직 새 선택적 필드, 새 enum 값, 새 메서드뿐입니다. 기존의 필수 필드, 방향, 제한, 오류 사유, 의미론은 절대 변경되지 않습니다. 표준 스키마와 그 SHA-256 해시는 `cmd/extension-protocol-gen`이 생성하며, CI의 `go test ./...`가 결정적 생성 테스트(`TestGeneratedArtifactsAreDeterministicAndCommitted`)를 통해 이를 강제합니다. 따라서 우발적인 의미 변경을 포함한 모든 드리프트(drift)는 빌드를 실패시킵니다.

## Security model

코드 확장은 **전체 신뢰(full trust)**입니다. Patty Code 샌드박스 외부에서 필터링되지 않은 상속 환경으로 실행되며, 전체 세션과 환경을 읽을 수 있고, 권한을 우회할 수 있으며, 머신을 직접 조작할 수 있습니다. `runtime` 블록이 있는 플러그인의 설치, 업데이트, 교체, 또는 `--link`가 곧 권한 부여이며 두 번째 확인 절차는 없습니다. 플러그인 흐름을 통해 설치된 플러그인(`plugin-packages.json`에 기록됨)만 사이드카를 시작할 수 있으며, 프로젝트 구성으로는 절대 선언할 수 없습니다. 사이드카 진단, 구조화된 UI, 인터셉터 사유, 공급자 오류가 UI, 로그, 오류 서피스에 도달하기 전에 호스트는 자격 증명 삭제(redaction) 패스를 실행합니다. 일반적인 공급자/모델 콘텐츠는 제품 데이터로 보존됩니다. 설치 미리보기, 플러그인 상세 정보, 기능 진단에는 런타임 플러그인에 대한 FULL TRUST 블록이 항상 표시됩니다.
