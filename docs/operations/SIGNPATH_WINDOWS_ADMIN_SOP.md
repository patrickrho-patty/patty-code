# Patty Code Windows SignPath 구성 및 검수 SOP

본 문서는 SignPath 관리자, GitHub 저장소 관리자, Release Maintainer가
Patty Code Windows Authenticode 2단계 서명 파이프라인을 구성하고 검수하는 데 사용합니다.

관련 변경 사항:

- PR: [pattycorp/DeepSeek-PattyCode#6904](https://github.com/pattycorp/DeepSeek-PattyCode/pull/6904)
- 기존 Preview 채널 개편(호환성 배경 참고용): [pattycorp/DeepSeek-PattyCode#6155](https://github.com/pattycorp/DeepSeek-PattyCode/pull/6155)
- 이 SOP의 검수 대상: 실행할 때마다 PR API로 읽어온 현재 PR Head
- 서명 워크플로: `.github/workflows/release-stable.yml`,
  `.github/workflows/release-desktop.yml`
- 머신 계약: `.signpath/contracts/release-signing.yml`
- Authenticode 검증 스크립트: `scripts/verify-windows-authenticode.ps1`

> PR Head가 이미 변경된 경우, 새로운 commit과 workflow diff를 반드시 다시
> 검토해야 하며, 본 문서에 기록된 이전 SHA로 공식 Secrets 검증을 계속해서는
> 안 됩니다.

## 1. 목표 및 완료 기준

이번 구성에서는 2단계 Windows 서명을 완료해야 합니다:

1. `windows-payload`를 사용하여 설치 후 실제 디스크에 저장되는 6개 EXE에
   서명합니다.
2. `windows-installer-v2`로 이 6개 EXE가 서명되었는지 검증한 후, 최종 NSIS
   설치 프로그램에 서명합니다.

전체 통과 기준:

- `windows-payload`와 `windows-installer-v2`가 모두 SignPath에 가져와졌고 상태가
  `VALID`입니다.
- 기존 `windows-installer`는 여전히 존재하며 `DEFAULT` 상태를 유지하고,
  덮어쓰거나 삭제되지 않습니다.
- `release-signing`은 정식 인증서, Trusted Build System 및 Origin
  Verification을 사용합니다.
- `release-signing`은 인증서가 강제하는 SignPath 승인을 유지하되, 전용
  `CI builds` 계정이 해당 GitHub environment에서 승인을 받은 뒤 자동으로
  완료합니다. 릴리스 담당자는 GitHub에서 한 번만 승인하면 됩니다.
- `test-signing-ci-approval`과 `windows-installer-test-v2`는 내부 서명 검증
  전용으로만 유지하며, 공개 Desktop 릴리스 워크플로에서 참조해서는 안 됩니다.
- AMD64와 ARM64 모두 정식 인증서로 제로 릴리스 사전 점검을 완료해야 합니다.
- 첫 단일 채널 정식 릴리스에서 모든 Authenticode 서명이 `Status = Valid`여야
  합니다.
- Windows Defender 환경에서 설치, 시작, 업데이트, 제거가 모두 통과해야 합니다.

## 2. 관리자 역할 분담

| 역할 | 담당 업무 |
| --- | --- |
| SignPath 조직 관리자 / 프로젝트 Configurator | Artifact Configuration 가져오기, 서명 정책 및 CI User 권한 유지 관리 |
| GitHub 저장소 관리자 | Actions Secrets/Variables 유지 관리, 필요 시 공식 임시 검증 브랜치 생성 |
| Release Maintainer | GitHub `release` environment 승인 1회, 정식 버전 릴리스 및 검수 |

기존 유지관리자에게 SignPath 프로젝트 구성 권한이 없다면, 조직 관리자가 직접
가져오기를 수행하거나 프로젝트 설정에서 유지관리자 또는 유지관리자 그룹을
`Configurators`에 추가할 수 있습니다.

SignPath 권한 설명:
[Users and permissions](https://docs.signpath.io/users/)

## 3. 현재 구성 기준선

2026-07-25 기준 온라인 확인 결과:

- SignPath 조직: `DeepSeek-Patty Code [OSS]`
- SignPath 프로젝트: `DeepSeek-PattyCode`
- 프로젝트 상태: `VALID`
- Repository URL:
  `https://github.com/pattycorp/DeepSeek-PattyCode.git`
- 현재 Artifact Configurations:
  - `Initial version`
  - `windows-installer`, 상태 `DEFAULT`
  - `windows-installer-test-v2`
  - `windows-installer-v2`
  - `windows-payload`
- `release-signing`에 Trusted Build System과 Origin Verification이 활성화되어
  있습니다.
- `release-signing`에서 `Use approval process`가 활성화되어 있으며 Required
  approvals는 `1`입니다.
- `release-signing`의 Allowed build definitions는 다음을 정확히 허용합니다:
  - `.github/workflows/release-stable.yml`
  - `.github/workflows/release-desktop.yml`
- `release-signing`의 Allowed branches는 정확히 `main-v2`여야 하며, 정식
  버전 태그는 최소 relay workflow가 이 보호된 제어면으로 전달합니다.
- `test-signing-ci-approval`은 테스트 인증서를 사용하며 `CI builds`만 제출 및
  승인할 수 있고, Required approvals는 `1`이며 동일한 Trusted Build, Origin 및
  Build Definition 제한이 적용됩니다.
- `Release certificate 2026`은 인증서 Restrictions에서
  `Requires approval process`를 활성화했으므로 이 승인은 프로젝트 정책에서
  끌 수 없습니다.

## 4. SignPath 구성 권한 부여

기존 조직 관리자가 직접 가져오기를 수행한다면 이 절은 건너뛸 수 있습니다.

1. SignPath에 로그인합니다.
2. `Projects`로 이동합니다.
3. `DeepSeek-PattyCode`를 엽니다.
4. 프로젝트 편집 또는 프로젝트 권한 설정으로 이동합니다.
5. 서명 구성을 유지 관리하는 사용자 또는 사용자 그룹을 `Configurators`에
   추가합니다.
6. 저장합니다.
7. 프로젝트를 다시 엽니다.
8. `Artifact Configurations` 영역에 `Add` 버튼이 나타나는지 확인합니다.

장기간 단일 계정에 묶어 두지 말고 유지관리자 그룹에 권한을 부여하는 것이
좋습니다.

## 5. `windows-payload` 가져오기

### 5.1 고정 버전 XML 확보

검토된 PR commit에서 파일을 복사해야 하며, XML을 수동으로 다시 작성하지
마세요:

- 저장소 경로:
  `.signpath/artifact-configurations/windows-payload.xml`
- 고정 버전:
  [windows-payload.xml@fe354e5](https://github.com/SivanCola/DeepSeek-PattyCode/blob/fe354e59a9a076930403b7d8aefb0bcd0b4e182a/.signpath/artifact-configurations/windows-payload.xml)

### 5.2 가져오기 절차

1. SignPath → `Projects` → `DeepSeek-PattyCode`.
2. `Artifact Configurations`를 찾습니다.
3. `Add`를 클릭합니다.
4. `Custom`을 선택합니다.
5. 이름 입력:

   ```text
   windows-payload
   ```

6. Slug는 반드시:

   ```text
   windows-payload
   ```

7. 위 파일의 전체 XML을 붙여넣습니다.
8. 저장합니다.
9. 구성 상태가 `VALID`인지 확인합니다.
10. `Open XML`을 클릭하여 온라인 XML이 저장소 파일과 한 글자까지 일치하는지
    대조합니다.
11. 이 구성을 `DEFAULT`로 설정하지 마세요.

### 5.3 구성 의미

GitHub `upload-artifact`가 SignPath에 제출하는 산출물은 ZIP이므로, 구성 루트
노드는 반드시 `<zip-file>`이어야 합니다.

이 구성은 다음 6개 EXE에 대해 `authenticode-sign`을 실행해야 합니다:

- `patty-desktop.exe`
- `patty-code-guard.exe`
- `patty-code-launcher.exe`
- `patty-code-update-helper.exe`
- `patty-code-cli.exe`
- `patty-code-uninstall.exe`

참고 자료:

- [Artifact configurations](https://docs.signpath.io/artifact-configuration/)
- [GitHub trusted build system](https://docs.signpath.io/trusted-build-systems/github)
- [Artifact configuration syntax](https://docs.signpath.io/artifact-configuration/syntax)

## 6. `windows-installer-v2` 가져오기

### 6.1 고정 버전 XML 확보

- 저장소 경로:
  `.signpath/artifact-configurations/windows-installer-v2.xml`
- 고정 버전:
  [windows-installer-v2.xml@fe354e5](https://github.com/SivanCola/DeepSeek-PattyCode/blob/fe354e59a9a076930403b7d8aefb0bcd0b4e182a/.signpath/artifact-configurations/windows-installer-v2.xml)

### 6.2 가져오기 절차

1. `Artifact Configurations → Add → Custom`을 다시 클릭합니다.
2. 이름 입력:

   ```text
   windows-installer-v2
   ```

3. Slug는 반드시:

   ```text
   windows-installer-v2
   ```

4. 위 파일의 전체 XML을 붙여넣습니다.
5. 저장합니다.
6. 구성 상태가 `VALID`인지 확인합니다.
7. `Open XML`을 클릭하여 온라인 XML이 저장소 파일과 한 글자까지 일치하는지
   대조합니다.
8. 이 구성을 `DEFAULT`로 설정하지 마세요.

### 6.3 구성 의미

이 구성에는 다음이 필요합니다:

1. 최종 `*installer*.exe`에 대해 `authenticode-sign`을 실행합니다.
2. 위 6개 내부 EXE에 대해 `authenticode-verify`를 실행합니다.

내부 EXE 중 하나라도 서명되지 않았거나 서명 후 다시 수정된 경우, 2단계
요청은 반드시 실패해야 하며 릴리스 가능한 설치 프로그램 생성을 계속해서는
안 됩니다.

참고 자료:

- [Artifact configuration reference](https://docs.signpath.io/artifact-configuration/reference)
- [Projects and versioned configurations](https://docs.signpath.io/projects)

## 7. 기존 구성 유지

가져오기가 완료된 후 Artifact Configurations는 다음과 같아야 합니다:

| 구성 | 예상 상태 |
| --- | --- |
| `Initial version` | 유지 |
| `windows-installer` | 유지하고 계속 `DEFAULT`로 둠 |
| `windows-payload` | 신규 추가, `VALID`, `DEFAULT` 아님 |
| `windows-installer-v2` | 신규 추가, `VALID`, `DEFAULT` 아님 |

다음 작업은 금지됩니다:

- `windows-installer` 삭제.
- `windows-installer`의 XML을 새 구성으로 교체.
- 기존 구성의 Slug 수정.
- 새 구성 두 개를 DEFAULT로 설정.

새 워크플로는 명시적인
`artifact-configuration-slug`로 구성을 선택하므로 기본 구성을 변경할 필요가
없습니다. 기존 구성을 유지하는 이유는 이전 release ref를 계속 다시 실행할 수
있도록 보장하기 위함입니다.

## 8. 서명 정책 확인 및 수정

### 8.1 `test-signing`

`test-signing`을 열고 다음을 확인합니다:

- 테스트 인증서를 사용합니다.
- Submitters에 `CI builds`가 포함됩니다.
- CI 요청은 자동으로 완료될 수 있으며 수동 승인이 필요하지 않습니다.
- Origin Verification을 활성화한 경우 저장소 주소는 반드시:

  ```text
  https://github.com/pattycorp/DeepSeek-PattyCode.git
  ```

### 8.2 `release-signing`

`release-signing → Edit`을 열고 다음을 설정합니다:

- Purpose: `Release signing`
- Certificate: 정식 Release certificate
- Submitters: 반드시 `CI builds`를 포함
- `Require trusted build system`: 켜기
- `Verify origin policy`: 켜기
- Repository URL:

  ```text
  https://github.com/pattycorp/DeepSeek-PattyCode.git
  ```

- Allowed branches: **`main-v2`만 입력 가능**
- Allowed build definitions: **다음 두 개의 정확한 경로만 줄 단위로 입력
  가능**:

  ```text
  .github/workflows/release-stable.yml
  .github/workflows/release-desktop.yml
  ```

  `.github/workflows/release-*.yml` 와일드카드를 사용해서는 안 되며, dispatch
  전달만 담당하는 trigger workflow를 추가해서도 안 됩니다. 저장소 내
  `.signpath/contracts/release-signing.yml`은 이 목록의 기계 판독 가능한 사실
  소스입니다. CI는 workflow 호출(call) 그래프를 파싱하여 새로운 최상위 서명
  진입점을 발견하면 실패로 차단합니다.
- `Use approval process`: **계속 켜기**
- Required approvals: `1`
- Approvers: 정식 릴리스를 처리할 수 있는 SignPath 수동 승인자가 최소한
  포함되어야 합니다

저장 후 정책을 다시 열어 다음을 확인합니다:

- 정책 상태가 `VALID`입니다.
- `Use approval process`가 켜져 있습니다.
- Trusted Build System과 Origin Verification이 여전히 켜져 있습니다.
- Allowed branches가 정확히 `main-v2`로 표시되며, `**`, `v*`,
  `desktop-v*` 또는 임시 테스트 브랜치가 없습니다.
- Allowed build definitions가 저장소 머신 계약과 항목별로 동일하며
  와일드카드가 없습니다.

정식 버전의 `vX.Y.Z` 태그 이벤트는 `release-stable-trigger.yml`이 전달합니다.
relay는 후보 tag만 전달하며, 실제 최상위 릴리스 workflow는 보호된 `main-v2`에서
고정 실행된 뒤 `vX.Y.Z`, `npm-vX.Y.Z`, `desktop-vX.Y.Z`가 공통으로 가리키는
불변 후보 SHA에 서명합니다.
Allowed branches를 태그 와일드카드로 변경해서는 안 됩니다. 일반 브랜치도
`v-malicious`와 같은 형태의 이름을 가질 수 있기 때문입니다.

`Release certificate 2026`의 Restrictions는 이 인증서를 사용하는 모든 정책이
승인 절차를 활성화하도록 명시적으로 요구합니다. 끄려고 하면 SignPath가 저장을
거부하며 다음 메시지를 표시합니다:

```text
Certificate requires an approval process.
You can either enable the approval process or use another certificate.
```

따라서 정식 서명 정책의 승인을 끄면 안 됩니다. 워크플로는 먼저
`wait-for-completion: false`로 요청을 제출하여 Signing Request ID를 획득한 뒤,
전용 `CI builds` 계정이 SignPath `Approve` API를 호출(call)하고 폴링하여 서명
산출물을 다운로드합니다. 정식 버전은 `release-signing`을 사용하며, 테스트
인증서는 산출물을 릴리스하지 않는 독립적인 내부 검증에만 사용됩니다.

## 9. GitHub Actions 구성 확인

이동 경로:

`Settings → Secrets and variables → Actions`

다음 Repository Secrets가 존재하는지 확인합니다:

- `SIGNPATH_API_TOKEN`
- `SIGNPATH_ORGANIZATION_ID`

보안 요구 사항:

- 로그, 스크린샷, Issue, PR 댓글 또는 채팅에 Secret 값을 표시해서는 안
  됩니다.
- Token에 해당하는 SignPath CI User는 `CI builds`여야 합니다.
- `CI builds`는 `release-signing`의 Submitter, Approver 권한을 반드시
  보유해야 합니다. 독립적인 내부 서명 검증이 여전히 테스트 정책을 사용하는
  경우에만 `test-signing-ci-approval` 권한을 추가로 부여합니다.
- 개인 Interactive User의 Token을 `SIGNPATH_API_TOKEN`으로 사용해서는 안
  됩니다.
- `SIGNPATH_ORGANIZATION_ID`는 올바른 OSS 조직을 가리켜야 합니다.
- GitHub `release` environment의 승인자가 여전히 유효해야 합니다.
- `release-signing`의 Allowed branches가 정확히 `main-v2`여야 합니다.

정식 검수 전에 서명 계약 attestation을 무효화해야 합니다:

```bash
gh variable set SIGNPATH_RELEASE_SIGNING_ATTESTATION \
  --repo pattycorp/DeepSeek-Patty Code \
  --body unverified
```

이렇게 하면 이전 attestation에 의존하는 모든 standalone 과거 경로가 실패로
차단됩니다. 정식 버전은 이전 attestation의 통과 여부를 읽지 않고, 같은
승인된 실행 안에서 먼저 실제 서명 사전 점검을 완료한 뒤 성공해야만 CLI,
npm 및 Desktop 릴리스를 시작합니다.

## 10. AMD64/ARM64 정식 인증서 제로 릴리스 사전 점검 실행

Fork PR 워크플로는 공식 저장소의 SignPath Secrets를 가져올 수 없으므로 PR
브랜치에서 직접 실제 서명을 완료할 수 없습니다. 병합 전 검증을 위해
`release-signing`의 정확한 `main-v2` 브랜치 제한을 완화하지 마세요. 코드,
workflow 계약 및 Secrets 없는 패키징 테스트가 PR에서 통과한 뒤, 보호된
`main-v2`에 병합한 다음 정식 인증서 사전 점검을 실행합니다.

제로 릴리스 사전 점검은 `.github/workflows/release-stable.yml`이 유일한
`release` environment에서 승인을 받은 후 자동으로 호출(call)합니다.
`release-signing`과 정식 인증서로 AMD64/ARM64를 검증하며, `CI builds`가
SignPath 요청을 자동 승인하고 publish job은 건너뜁니다. 요청 4개가 모두
성공하면 현재 계약 지문이 `SIGNPATH_RELEASE_SIGNING_ATTESTATION`에
기록됩니다.

```bash
./scripts/release-stable.sh X.Y.Z
```

이 명령은 먼저 원격 `main-v2`의 reviewed Notes와 정확한 SHA CI를 검증한 뒤,
세 개의 정식 버전 태그를 원자적으로 생성(create)합니다. 이후 relay가 보호된
제어면을 시작합니다. 사전 점검이 완료되기 전에는 CLI, npm 및 Desktop 세 개의
공개 publisher가 모두 시작되지 않으므로, SignPath 정책 변경이 반쪽 릴리스로
이어지지 않습니다.
사전 점검을 위해 Preview/RC 태그나 `canary` environment를 다시 도입하지
마세요.

### 10.1 실행 모니터링

```bash
RUN_ID="$(gh run list \
  --repo pattycorp/DeepSeek-Patty Code \
  --workflow release-stable.yml \
  --branch main-v2 \
  --event workflow_dispatch \
  --limit 1 \
  --json databaseId \
  --jq '.[0].databaseId')"

gh run watch "$RUN_ID" \
  --repo pattycorp/DeepSeek-Patty Code \
  --exit-status
```

## 11. 정식 인증서 사전 점검 검수 기준

다음 두 작업이 동시에 성공해야 합니다:

- `build (windows-amd64)`
- `build (windows-arm64)`

각 플랫폼은 다음을 완료해야 합니다:

1. 서명되지 않은 payload를 빌드합니다.
2. payload를 업로드합니다.
3. `windows-payload`로 6개 EXE에 서명합니다. `CI builds`가 승인을 자동으로
   기록합니다.
4. 서명된 payload로 portable ZIP과 NSIS 설치 프로그램을 다시 생성합니다.
5. installer signing bundle을 업로드합니다.
6. `windows-installer-v2`로 내부 신뢰 서명을 검증하고 외부 설치 프로그램에
   서명합니다.
7. Authenticode release contract 검증을 실행합니다.
8. `publish` job은 `signing_preflight=true`로 인해 건너뜁니다.

SignPath Signing Requests에 성공한 요청 4개가 나타나야 합니다:

| 아키텍처 | 1단계 | 2단계 |
| --- | --- | --- |
| AMD64 | payload 서명 | installer 검증 및 서명 |
| ARM64 | payload 서명 | installer 검증 및 서명 |

항목별로 확인합니다:

- 상태가 `Completed`입니다.
- Artifact Configuration Slug가 올바릅니다.
- Origin이 공식 저장소를 가리킵니다.
- Commit SHA가 GitHub Actions 실행 SHA와 일치합니다.
- Trusted Build, Origin Verification, Malware Scan이 모두 통과합니다.
- 자동 사전 점검과 일반 릴리스 요청의 승인 Actor가 모두 `CI builds`입니다.
- AMD64와 ARM64의 payload, portable ZIP 내 파일 및 최종 installer가 모두
  `Status = Valid` 신뢰 체인 검증을 통과합니다.

## 12. 정식 서명 attestation 확인

다음 조건이 모두 충족된 경우에만 활성화할 수 있습니다:

- 새 Artifact Configuration 두 개가 모두 `VALID`입니다.
- 기존 `windows-installer`가 변경되지 않았습니다.
- `release-signing`의 인증서 수준 승인이 켜진 상태로 유지되며 정식 승인자를
  사용할 수 있습니다.
- `release-signing`의 Build Definitions가
  `.github/workflows/release-stable.yml`,
  `.github/workflows/release-desktop.yml`을 정확히 허용합니다.
- `release-signing`의 Allowed branches가 정확히 `main-v2`입니다.
- `CI builds`가 `release-signing`의 Submitter 및 Approver이며, GitHub Secret은
  전용 Token을 사용합니다.
- AMD64와 ARM64 정식 인증서 제로 릴리스 사전 점검이 모두 성공했습니다.
- SignPath Signing Request 4개가 모두 성공했습니다.

사전 점검은 변수를 자동으로 기록하므로, 다시 읽어 확인만 하면 됩니다:

```bash
gh variable get SIGNPATH_RELEASE_SIGNING_ATTESTATION \
  --repo pattycorp/DeepSeek-PattyCode
```

값은 `v1:` 뒤에 64자리 소문자 16진수 SHA-256이 붙은 형태여야 합니다.
workflow, 서명 스크립트, Artifact Configuration 또는 머신 계약이 변경되면
CI가 계산한 새 지문이 더 이상 일치하지 않으므로, 제로 릴리스 사전 점검을
다시 실행해야 합니다.

## 13. 첫 단일 채널 정식 버전 검수

단일 채널 릴리스는 더 이상 공개 RC를 생성(create)하지 않습니다.
`signing_preflight=true`가 유일한 정식 버전 run 안에서 정식 인증서, 2단계
산출물 체인 및 자동 승인 폐쇄 루프가 올바르다는 것을 증명하며, 성공한
경우에만 같은 run의 공개 publisher 시작이 허용됩니다. 처음 전환한 버전은
본 문서의 11, 12, 14절과 저장소 `docs/operations/RELEASING.md`의 교차 표면(cross-surface)
postflight를 완전히 수행해야 하며, 추가 prerelease로 대체해서는 안 됩니다.

## 14. 정식 인증서 및 Defender 검수

정식 버전 Windows 검증은 `RequireTrusted=true`를 활성화합니다. 모든
Authenticode 서명은 다음을 반환(return)해야 합니다:

```text
Status = Valid
```

깨끗한 Windows 11 AMD64 및 ARM64 환경에서 설치 디렉터리를 확인합니다:

```powershell
Get-ChildItem "<Patty Code 설치 디렉터리>" -Recurse -Filter *.exe |
  ForEach-Object {
    $signature = Get-AuthenticodeSignature $_.FullName
    [PSCustomObject]@{
      File    = $_.FullName
      Status  = $signature.Status
      Subject = $signature.SignerCertificate.Subject
    }
  }
```

검수 요구 사항:

- 설치 프로그램 서명이 `Valid`입니다.
- 설치 디렉터리 내 6개 EXE 전체의 서명이 `Valid`입니다.
- Portable ZIP 내 실행 파일의 서명이 `Valid`입니다.
- AMD64와 ARM64의 서명 인증서 Subject가 예상과 일치합니다.
- Windows Defender가 켜진 상태로 유지됩니다.
- 설치, 최초 시작, CLI 호출(call), 업데이트, 제거를 실제로 완료합니다.
- Windows Security → Protection History에 새로운 격리 또는 차단이 없습니다.

AMD64와 ARM64가 모두 통과해야 첫 단일 채널 정식 버전의 검수가 완료된
것으로 간주합니다.

## 15. 장애 처리 및 롤백

| 증상 | 조치 |
| --- | --- |
| `Add`가 보이지 않음 | 프로젝트 Configurator 추가, 또는 조직 관리자가 직접 가져오기 |
| XML을 저장할 수 없거나 상태가 `VALID`가 아님 | 새 구성만 수정하고 기존 `windows-installer`는 변경하지 않음 |
| 내부 Test Signing Request가 오래 Pending 상태 | 테스트 정책 승인 구성, CI User, 구성 Slug 및 요청 오류 확인. 공개 workflow는 테스트 정책으로 전환해 문제를 우회하지 않음 |
| Release Signing Request가 Pending approval 표시 | 정식 인증서의 강제 게이트이며, 권한 있는 SignPath 승인자가 Action 타임아웃 전에 처리 |
| Origin Verification 실패 | 저장소 URL, ref, SHA 및 GitHub Trusted Build 확인 |
| 파일 누락 또는 추가 파일 발견 | signing bundle과 XML 파일 목록이 일치하는지 확인 |
| `authenticode-verify` 실패 | 내부 파일이 서명되지 않았거나 서명 후 다시 컴파일/수정되었는지 확인 |
| AMD64만 성공 | 통과시키지 않음, ARM64도 필수 게이트 |
| 정식 버전 서명이 `Status = Valid`가 아님 | attestation을 `unverified`로 설정하고 이후 릴리스를 중단한 뒤, 불변 태그 기준으로 누락된 표면 복구 |
| 자동 사전 점검 요청이 수동 승인 대기 중 | attestation을 `unverified`로 설정하고 `CI builds` Approver 권한, API Token 및 자동 승인 단계 확인. 인증서 강제 승인은 끄지 않음 |

정식 서명 장애가 발생하면 즉시 실패 차단을 복구합니다:

```bash
gh variable set SIGNPATH_RELEASE_SIGNING_ATTESTATION \
  --repo pattycorp/DeepSeek-Patty Code \
  --body unverified
```

장애가 해소되고 AMD64/ARM64 검수를 다시 완료하기 전에는 안정 버전을
릴리스해서는 안 됩니다.

## 16. 최종 서명 체크리스트

- [ ] `windows-payload`가 가져와졌고 `VALID`입니다
- [ ] `windows-installer-v2`가 가져와졌고 `VALID`입니다
- [ ] 기존 `windows-installer`가 여전히 존재하며 `DEFAULT`를 유지합니다
- [ ] 공개 릴리스 workflow가 `test-signing-ci-approval` 또는 `windows-installer-test-v2`를 참조하지 않습니다
- [ ] `CI builds`가 `release-signing`의 Submitter, Approver입니다
- [ ] `SIGNPATH_API_TOKEN`이 전용 `CI builds`에 해당하며 개인 계정이 아닙니다
- [ ] `release-signing`에 Trusted Build System이 켜져 있습니다
- [ ] `release-signing`에 Origin Verification이 켜져 있습니다
- [ ] `release-signing`의 Allowed build definitions가 정확히 `.github/workflows/release-stable.yml` 및 `.github/workflows/release-desktop.yml`입니다
- [ ] `release-signing`의 Allowed branches가 정확히 `main-v2`입니다
- [ ] `release-signing`의 SignPath 승인이 켜져 있으며 Required approvals가 `1`입니다
- [ ] GitHub `release` environment의 정식 릴리스 승인자와 대응 절차가 명확히 정해져 있습니다
- [ ] AMD64 정식 인증서 제로 릴리스 사전 점검 2단계 서명이 성공했습니다
- [ ] ARM64 정식 인증서 제로 릴리스 사전 점검 2단계 서명이 성공했습니다
- [ ] SignPath Signing Request 4개가 모두 `Completed`입니다
- [ ] `SIGNPATH_RELEASE_SIGNING_ATTESTATION`이 현재 머신 계약 지문과 일치합니다
- [ ] 첫 단일 채널 정식 버전의 AMD64 서명이 모두 `Valid`입니다
- [ ] 첫 단일 채널 정식 버전의 ARM64 서명이 모두 `Valid`입니다
- [ ] Defender 설치, 시작, 업데이트, 제거 검증이 통과했습니다
- [ ] 서명된 안정 버전이 실제로 다운로드 가능해진 후에 해당 이슈를 닫습니다

## 17. 참고 자료

- [SignPath Artifact Configuration](https://docs.signpath.io/artifact-configuration/)
- [SignPath Artifact Configuration Syntax](https://docs.signpath.io/artifact-configuration/syntax)
- [SignPath Artifact Configuration Reference](https://docs.signpath.io/artifact-configuration/reference)
- [SignPath Projects](https://docs.signpath.io/projects)
- [SignPath Users and Permissions](https://docs.signpath.io/users/)
- [SignPath GitHub Trusted Build System](https://docs.signpath.io/trusted-build-systems/github)
- [Patty Code PR #6904](https://github.com/pattycorp/DeepSeek-PattyCode/pull/6904)
