# GongCode Comprehensive Product, Architecture, Security, Governance, Infrastructure, and Public Website Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use a task-by-task implementation workflow with independent review gates. Every phase in this document must produce deployable, testable software and evidence before the next phase begins.

**Document status:** Product and implementation master plan  
**Product name:** **GongCode / 공코드**  
**Korean positioning:** **공공기관을 위한 보안형 AI 개발체계**  
**English positioning:** **A sovereign, governed, and auditable AI software engineering platform for the public sector**  
**Prepared:** 2026-08-04  
**Primary deployment environments:** Closed network, air-gapped network, private GPU infrastructure, on-premises government data center, and separately certified CSAP cloud deployment  
**Website scope in this plan:** 128 public pages

---

## 1. Executive Summary

GongCode is not only a coding assistant. It is a controlled engineering system for Korean government agencies, public institutions, regulated enterprises, and system integrators that must use AI without surrendering code, prompts, personal information, operational data, model control, or auditability.

The product consists of a secure coding harness, a governed model and GPU platform, disposable execution sandboxes, deterministic policy enforcement, real-time administration and security operations, line-level AI provenance, evidence generation, and public-sector-specific engineering intelligence.

The principal product promise is:

> **Develop faster with AI while every context item, prompt, model decision, tool action, code change, approval, security result, and exported artifact remains authorized, attributable, reviewable, and reproducible.**

GongCode assumes the model can be wrong, manipulated, compromised, or induced to request dangerous actions. Security therefore does not depend on prompt wording or model obedience. Identity, file access, context assembly, command execution, network access, package installation, credentials, model selection, and artifact export are enforced by independent services outside the model.

The plan deliberately distinguishes three things that are often mixed together:

1. **Coding intelligence:** understanding repositories, generating and reviewing code, debugging, testing, and explaining systems.
2. **Execution authority:** what the agent is technically permitted to read, change, execute, connect to, or export.
3. **Governance evidence:** how the organization proves who did what, under which policy, with which model, against which code and data, and with which human approvals.

No marketing page or product screen may claim that GongCode is “CSAP compliant,” “KISA certified,” “ISMS-P compliant,” or legally compliant merely because controls exist. The system will provide control mappings, evidence, configuration profiles, and certification-ready operations. Formal certification and legal conclusions require the relevant assessment, deployment scope, operational procedures, and authority.

---

## 2. Goals

### 2.1 Product goals

- Provide a Claude Code-, Codex-, or Pi-class coding experience entirely inside approved Korean government infrastructure.
- Support terminal-first, IDE-integrated, and controlled web-based workflows.
- Operate with no public internet dependency.
- Use customer-owned or approved private GPUs.
- Enforce policy before context disclosure, tool execution, model invocation, and artifact export.
- Make AI-assisted code attributable at the repository, commit, file, AST node, line, and surviving-code-span levels.
- Give administrators a real-time operational view of every connected harness, prompt exchange, model request, token flow, sandbox, GPU, policy decision, alert, and approval.
- Make Korean public-sector requirements first-class through versioned policy packs, retrieval sources, model evaluations, and engineering templates.
- Produce audit and certification evidence continuously rather than reconstructing it after an incident.
- Support deployment in air-gapped, closed-network, on-premises, CSAP-cloud, and hybrid environments without changing the product’s core control model.
- Publish a technically deep public website that earns trust through specificity rather than generic “secure AI” language.

### 2.2 Security goals

- A fully compromised coding model must not escape the authorized task boundary.
- A malicious repository file or prompt-injection instruction must not grant additional privileges.
- The platform must never give the model direct GPU administration, Kubernetes administration, unrestricted shell, host sockets, long-lived credentials, or production access.
- All high-risk actions must be preventable through deterministic policy.
- Secrets and personal information must be detected before entering model context and before leaving the sandbox.
- Logs and provenance must be tamper-evident and stored outside the sandbox.
- Every artifact leaving a sandbox must pass an export gate.
- Every model, policy pack, sandbox image, tool, and dependency mirror must have an approval and expiry state.

### 2.3 Commercial goals

- Sell one coherent platform rather than disconnected security appliances.
- Provide a procurement-friendly edition matrix for ministries, local governments, public enterprises, and system integrators.
- Use the same core codebase for on-premises and cloud deployments.
- Make compliance evidence, model governance, and provenance meaningful differentiators.
- Support paid implementation, policy-pack customization, model evaluation, sovereign deployment, and ongoing managed operations.

---

## 3. Non-goals

- GongCode will not autonomously deploy to production by default.
- GongCode will not rely on hidden model reasoning as compliance evidence.
- GongCode will not treat a model’s self-reported confidence as a security control.
- GongCode will not store every raw prompt indefinitely by default.
- GongCode will not permit unrestricted internet browsing from a code sandbox.
- GongCode will not fine-tune mutable laws, regulations, or current policy text into weights as the only source of truth.
- GongCode will not imply official government endorsement through branding, imagery, seals, flags, or domain naming.
- GongCode will not promise legal compliance without deployment-specific assessment.
- GongCode will not support anonymous usage in government deployments.
- GongCode will not require one proprietary model, GPU vendor abstraction, Git provider, or cloud provider.

---

## 4. Design Principles

1. **Trust is external to the model.** The model proposes; independently controlled services authorize and execute.
2. **Least privilege is task-scoped.** Permissions are derived for one user, repository, task, sandbox, model, and time window.
3. **No network is the default.** Connectivity is explicitly granted through named brokers.
4. **Context is a governed resource.** The model does not inherit all data the user can theoretically reach.
5. **Every action has provenance.** Prompts, tools, code spans, policies, and approvals are linked through immutable identifiers.
6. **Controls are composable.** Agencies can enable policy packs and stricter local rules without forking product code.
7. **Evidence is generated during operation.** Compliance reports are derived from signed events and configuration state.
8. **Human approval is risk-based.** Low-risk actions remain fast; material changes receive review.
9. **Closed-network usability matters.** Updates, documentation, packages, models, and licenses must work offline.
10. **Korean public-sector engineering is a product surface.** It is not merely translated UI.
11. **No silent degradation.** If a required scanner, policy service, or audit sink is unavailable, protected actions fail closed.
12. **Explainability is grounded.** Show inputs, retrieved evidence, tool calls, diffs, tests, and policy results rather than fabricated chain-of-thought.
13. **Accessibility and records management are built in.** Government usability includes keyboard operation, audit export, retention, and Korean language quality.
14. **Separation of duties is enforceable.** Platform administrators, security officers, auditors, model approvers, and developers have distinct powers.
15. **Security claims are measurable.** Every claim maps to a test, control, event, or evidence artifact.

---

## 5. Key Terminology

| Term | Definition |
|---|---|
| Harness | Developer-facing terminal, IDE, or web interface that plans and requests AI-assisted engineering actions. |
| Session | One identity-bound, repository-bound, policy-bound interaction lifecycle. |
| Sandbox | Disposable isolated environment in which approved code operations execute. |
| Context item | A file span, document, API schema, issue, log, or other data object proposed for model input. |
| Prompt exchange | The governed request and response envelope exchanged with a model endpoint. |
| Assurance Box | A small independent control component that evaluates, transforms, blocks, or records a specific class of action. |
| Policy pack | Versioned, signed set of rules, mappings, defaults, and evidence requirements. |
| Provenance span | A code range linked to the AI session, user, model, source context, tools, policy, tests, and approvals that created or modified it. |
| Evidence bundle | Signed package containing configuration, decisions, scans, logs, approvals, and artifact hashes for audit or certification. |
| Model approval | A time-bounded decision defining where a model may be used, for which data classification, and under which restrictions. |
| Closed-network update | Signed software, model, document, or policy bundle transferred through an approved media and import process. |

## 6. Product Suite and Component Naming

GongCode should be sold as one platform with clear modules. GongCode Control is the unified administrative platform; the security, governance, provenance, model, GPU, and audit functions appear as integrated workspaces rather than separate disconnected consoles.

### 6.1 Product-level names

| Product | Purpose | Primary users |
|---|---|---|
| **GongCode Harness** | Terminal, IDE, and controlled web coding agent experience | Developers, reviewers, architects |
| **GongCode Control** | Unified administration, operations, security, governance, provenance, GPU, model, and compliance console | Platform administrators, SOC, AI governance, auditors |
| **GongCode Guard** | Deterministic policy, DLP, prompt, context, command, package, and export enforcement services | Security and platform teams |
| **GongCode Runtime** | Disposable microVM and container execution platform | Harness and CI workloads |
| **GongCode Gateway** | Governed inference routing, quotas, token accounting, prompt controls, and model access | Harness, internal applications |
| **GongCode Registry** | Approved models, tools, policies, packages, sandbox images, prompts, and evaluation artifacts | Model approvers, security, platform teams |
| **GongCode Trace** | Code-level provenance, explainability, replay, human/AI attribution, and evidence linking | Developers, auditors, reviewers |
| **GongCode Evidence** | Tamper-evident logs, evidence bundles, retention, legal hold, and audit exports | Auditors, compliance, incident response |
| **GongCode Connect** | Git, CI/CD, issue tracker, artifact repository, identity, and document integrations | Platform engineers |
| **GongCode Evaluate** | Model, agent, security, Korean coding, policy, and regression evaluation | Model team, security, QA |
| **GongCode Update** | Signed offline update creation, review, import, rollback, and fleet distribution | Air-gap operators |
| **GongCode Public** | Internet-facing product, documentation, trust, and resources website | Prospects, procurement, partners |

### 6.2 GongCode Assurance Boxes

“Assurance Box” is the architectural name for an independently deployable or independently testable enforcement component. Each box consumes a signed action envelope and emits a signed `AssuranceDecision`.

| Assurance Box | Internal service name | Core responsibility | Default failure mode |
|---|---|---|---|
| **Identity Box** | `identity-authorizer` | Resolve user, organization, clearance, role, and delegated authority | Fail closed |
| **Device Box** | `device-posture` | Validate managed device, certificate, posture, and session binding | Fail closed |
| **Prompt Box** | `prompt-governor` | Classify, template, redact, retain, and authorize prompts | Fail closed for protected classes |
| **Context Box** | `context-firewall` | Decide which files, spans, docs, and metadata may reach the model | Fail closed |
| **Secrets Box** | `secret-sentinel` | Detect, tokenize, mask, quarantine, and prevent secret exposure | Fail closed |
| **PII Box** | `pii-shield` | Detect Korean and global identifiers, apply purpose and minimization rules | Fail closed |
| **Injection Box** | `injection-defender` | Identify untrusted instructions in repositories, docs, logs, and web content | Quarantine and downgrade trust |
| **File Box** | `file-guard` | Enforce path, repository, branch, classification, and read/write scopes | Fail closed |
| **Command Box** | `command-guard` | Parse and authorize shell, build, test, and tool operations | Fail closed |
| **Network Box** | `network-gate` | Enforce destination, protocol, direction, purpose, and rate limits | No network |
| **Package Box** | `package-gate` | Broker dependencies from approved mirrors and versions | Deny unknown package |
| **License Box** | `license-guard` | Enforce SPDX allow/deny policy and dependency obligations | Block prohibited license |
| **Crypto Box** | `crypto-guard` | Detect prohibited algorithms and require approved cryptographic patterns | Block prohibited use |
| **Model Box** | `model-authorizer` | Enforce model approval, data-classification limits, quotas, and expiry | Fail closed |
| **Runtime Box** | `runtime-warden` | Create, constrain, monitor, pause, snapshot, and destroy sandboxes | Terminate unsafe runtime |
| **Output Box** | `artifact-gate` | Scan patches, binaries, logs, reports, archives, and exports | Quarantine |
| **Provenance Box** | `trace-writer` | Link prompts, context, tools, code spans, tests, and approvals | Block protected merge if unavailable |
| **Evidence Box** | `evidence-builder` | Assemble signed control and session evidence bundles | Queue locally, then fail closed at threshold |
| **Logging Box** | `audit-forwarder` | Stream append-only events outside the sandbox | Fail closed for protected actions |
| **Response Box** | `response-inspector` | Scan generated text and code for leakage, unsafe patterns, and unsupported claims | Redact or block |
| **Resource Box** | `resource-governor` | Enforce CPU, memory, disk, process, token, GPU, and time budgets | Throttle or terminate |
| **Update Box** | `update-verifier` | Verify offline bundles, signatures, SBOMs, approvals, and rollback metadata | Reject bundle |

### 6.3 Naming shown to customers

Technical service names should remain stable internally. Customer-facing screens should use clear Korean labels:

- 신원 확인
- 기기 신뢰
- 프롬프트 관리
- 컨텍스트 방화벽
- 비밀정보 검사
- 개인정보 보호
- 프롬프트 인젝션 방어
- 파일 권한
- 명령 실행 통제
- 네트워크 통제
- 패키지 승인
- 라이선스 정책
- 암호화 정책
- 모델 승인
- 격리 실행
- 산출물 반출 심사
- AI 출처 기록
- 감사 증적
- 보안 로그
- 응답 검사
- 자원 통제
- 폐쇄망 업데이트

---

## 7. Primary Personas and Separation of Duties

| Persona | Allowed capabilities | Explicitly separated capabilities |
|---|---|---|
| Developer | Use Harness, request context, edit in sandbox, create candidate patch | Cannot approve own high-risk exceptions or modify global policies |
| Technical reviewer | Review diffs, tests, provenance, and architecture | Cannot erase evidence or approve model deployment |
| Project owner | Define project rules, repositories, reviewers, and budgets | Cannot bypass organization security baseline |
| Security operator | Investigate alerts, isolate sessions, revoke tokens, capture evidence | Cannot alter code or silently read unrestricted prompts |
| AI governance officer | Approve model use cases, evaluations, prompt policies, and risk tiers | Cannot operate GPU hosts or change source repositories |
| Model approver | Approve model/version/classification combinations | Cannot change evaluation results |
| Platform administrator | Operate clusters, sandboxes, gateways, and updates | Cannot modify audit records or self-approve privileged actions |
| Compliance manager | Select compliance profiles, map controls, request evidence | Cannot override security blocks |
| Auditor | Read signed evidence, provenance, and control state | Read-only; sensitive prompt access requires separate authorization |
| Incident commander | Coordinate containment, investigation, recovery, and reporting | Time-bounded emergency authority with complete recording |
| GPU operator | Drain nodes, manage capacity, deploy approved model images | Cannot inspect repository content or prompt text |
| Organization administrator | Manage tenants, users, groups, and local policy overlays | Cannot weaken mandatory central controls |
| Procurement observer | View architecture, certification status, release notes, and reports | No operational access |

### 7.1 Role implementation

Use both RBAC and ABAC:

- RBAC defines baseline job functions.
- ABAC evaluates agency, project, classification, device, location, network zone, repository, branch, data category, model approval, action risk, and time.
- Just-in-time elevation requires reason, approver, scope, duration, and automatic revocation.
- Break-glass access requires two-person authorization where available, mandatory recording, automatic incident creation, and post-use review.
- No shared user accounts.
- Service identities are workload-bound and certificate-based.

## 8. High-Level Architecture

### 8.1 Trust-zone architecture

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ ZONE 0 — INTERNET                                                           │
│ GongCode Public website, public documentation, trust center                │
│ No route to customer control planes, model zones, evidence, or repositories│
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │ controlled publishing only
┌──────────────────────────────────▼──────────────────────────────────────────┐
│ ZONE 1 — USER ACCESS                                                        │
│ Managed workstation, terminal harness, IDE extension, SSO, device cert     │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │ mTLS + identity-bound requests
┌──────────────────────────────────▼──────────────────────────────────────────┐
│ ZONE 2 — CONTROL PLANE                                                      │
│ Identity, session orchestration, policies, approvals, Control UI           │
│ No direct execution of generated code                                      │
└──────────────┬──────────────────────┬──────────────────────┬────────────────┘
               │                      │                      │
               │ signed tool request  │ model request        │ events
┌──────────────▼──────────────┐ ┌─────▼────────────────┐ ┌──▼────────────────┐
│ ZONE 3 — EXECUTION PLANE    │ │ ZONE 4 — MODEL PLANE │ │ ZONE 5 — EVIDENCE│
│ MicroVM sandboxes           │ │ Gateway + GPU fleet  │ │ WORM-style store │
│ Repo snapshots and tools    │ │ Approved model only  │ │ SIEM + provenance│
│ No direct broad network     │ │ No repo or shell     │ │ Separate admins  │
└──────────────┬──────────────┘ └─────┬────────────────┘ └──▲────────────────┘
               │                      │                     │ append-only
               │ brokered access      │ approved artifacts  │
┌──────────────▼──────────────────────▼─────────────────────┴────────────────┐
│ ZONE 6 — SUPPLY CHAIN AND INTEGRATIONS                                    │
│ Git broker, package mirrors, artifact registry, CI/CD, document sources   │
│ All access mediated, authenticated, scanned, and logged                   │
└────────────────────────────────────────────────────────────────────────────┘
```

### 8.2 Logical service architecture

```text
Developer
  │
  ├─ GongCode Harness CLI / IDE
  │       │
  │       ▼
  │  Session API ── Identity Box ── Device Box
  │       │
  │       ▼
  │  Agent Orchestrator
  │       ├─ Prompt Box
  │       ├─ Context Box ── Secrets Box ── PII Box ── Injection Box
  │       ├─ Model Box ── GongCode Gateway ── Approved GPU endpoint
  │       └─ Tool Broker
  │              ├─ File Box
  │              ├─ Command Box
  │              ├─ Network Box
  │              ├─ Package Box
  │              ├─ License Box
  │              └─ Runtime Box ── Disposable sandbox
  │
  ├─ Response Box
  ├─ Output Box
  ├─ Provenance Box
  ├─ Logging Box
  └─ Evidence Box
          │
          ▼
   GongCode Control
   ├─ Live Operations
   ├─ Security Operations
   ├─ Prompt Governance
   ├─ Policy Studio
   ├─ Compliance Center
   ├─ Provenance Explorer
   ├─ Model and GPU Control
   └─ Evidence and Audit
```

### 8.3 Architectural rule

The orchestrator is not trusted to bypass controls. It cannot call a model, read a file, execute a command, open a network connection, install a dependency, access a secret, or export an artifact without presenting a signed request to the relevant Assurance Box.

---

## 9. End-to-End Session Flow

1. User authenticates through organization SSO and device certificate.
2. Harness requests a new session for a named organization, project, repository, branch, and task.
3. Identity Box and Device Box establish the actor and device trust.
4. Session service resolves:
   - project classification,
   - mandatory policy packs,
   - allowed models,
   - repository scope,
   - initial tool capabilities,
   - token and resource budget,
   - required approval rules,
   - retention profile.
5. Git broker produces an immutable repository snapshot at a specific commit.
6. Runtime Box launches a disposable sandbox from a signed image.
7. The harness submits the user request to Prompt Box.
8. Prompt Box classifies the request, applies templates, redaction, retention, and risk labels.
9. The orchestrator proposes context.
10. Context Box evaluates every file, span, issue, document, schema, or log.
11. Secrets Box and PII Box remove or tokenize protected values.
12. Injection Box labels untrusted instruction-like content and prevents it from altering authority.
13. Model Box selects an approved model endpoint based on classification, task, language, capacity, and model approval.
14. Gateway sends a sanitized prompt envelope to the model.
15. The model proposes text or tool calls.
16. Each tool call is independently evaluated by File, Command, Network, Package, License, Crypto, Resource, and Runtime Boxes as applicable.
17. The sandbox executes only the approved operation.
18. Tool results pass through Response Box before returning to model context.
19. Code changes remain inside the sandbox.
20. Tests, scanners, and policy checks run.
21. Provenance Box maps generated and modified code spans to session inputs and actions.
22. Output Box evaluates the candidate patch and all exported artifacts.
23. Required human approvals are collected.
24. GongCode Connect creates a signed commit candidate or pull request.
25. Evidence Box packages the policy decisions, provenance, tests, scans, approvals, and hashes.
26. Sandbox is destroyed after export and retention requirements are satisfied.

### 9.1 Signed action envelope

Every protected action uses a common envelope:

```json
{
  "action_id": "act_01J...",
  "session_id": "ses_01J...",
  "actor_id": "usr_...",
  "device_id": "dev_...",
  "organization_id": "org_...",
  "project_id": "prj_...",
  "repository_id": "repo_...",
  "commit_sha": "sha256-or-git-commit",
  "classification": "restricted",
  "action_type": "file.read",
  "resource": "src/auth/JwtProvider.java",
  "purpose": "refactor authentication validation",
  "requested_capabilities": ["read"],
  "policy_bundle_hash": "sha256:...",
  "sandbox_id": "sbx_...",
  "timestamp": "RFC3339",
  "nonce": "..."
}
```

Every Assurance Box returns:

```json
{
  "decision_id": "dec_01J...",
  "action_id": "act_01J...",
  "box": "context-firewall",
  "result": "allow_with_transform",
  "reason_codes": ["PATH_ALLOWED", "SECRET_VALUES_TOKENIZED"],
  "obligations": ["RETAIN_1Y", "REDACT_PROMPT_VIEW"],
  "policy_rule_ids": ["GC-CONTEXT-004", "GC-PII-012"],
  "policy_bundle_hash": "sha256:...",
  "decision_timestamp": "RFC3339",
  "signature": "..."
}
```

---

## 10. GongCode Harness

### 10.1 Interfaces

GongCode Harness has three interfaces that use the same session and policy APIs:

1. **CLI:** primary interface for closed-network developers.
2. **IDE extension:** VS Code first; Eclipse/eGovFrame support follows; JetBrains support after core stabilization.
3. **Controlled web workspace:** for training, demonstrations, constrained review, and environments where local extensions are prohibited.

### 10.2 Core developer features

- Repository onboarding and project-policy discovery.
- Task planning with explicit proposed files, commands, network requirements, packages, and risk level.
- Repository semantic map with symbols, dependencies, build modules, APIs, database access, and authorization boundaries.
- Natural Korean and English interaction.
- Context preview before model invocation.
- “Why is this file needed?” explanation for every context request.
- User ability to remove context but not force unauthorized context.
- Read, edit, patch, search, build, test, lint, format, and review tools.
- Multi-file refactoring.
- Test generation and failure analysis.
- Debug-log summarization with secret and PII controls.
- Migration assistance for eGovFrame, Spring, Java, MyBatis, SQL, batch, legacy JSP, and modern APIs.
- Dependency requests through Package Box rather than arbitrary internet install.
- Human approval prompts at the exact point of risk.
- Candidate patch review before export.
- AI provenance badges inside diff views.
- Offline retrieval from approved documentation.
- Project memory with scope, expiry, owner, and review.
- Reproducible session snapshots.
- Session handoff to another authorized reviewer.
- Controlled multi-agent review for security, architecture, testing, and compliance.
- Rate and token visibility for users.
- Clear display of selected model, model approval status, and policy profile.
- Local command suggestions that never execute until authorized.
- Protected-branch awareness.
- Safe rollback within sandbox.
- Structured problem-report export without sensitive repository content.

### 10.3 Harness task modes

| Mode | Purpose | Tool authority |
|---|---|---|
| Ask | Explain code and answer questions | Read-only, tightly scoped context |
| Plan | Produce an execution plan | Read-only; proposed actions shown |
| Edit | Modify files in sandbox | File write within approved paths |
| Test | Build and run tests | Approved commands, no network by default |
| Review | Review diff, commit, or pull request | Read-only plus scanners |
| Migrate | Perform controlled framework or dependency migration | Higher approval threshold |
| Investigate | Analyze failures and logs | Sanitized logs, no production credentials |
| Secure | Run security-focused remediation | Security scans and restricted code changes |
| Document | Generate controlled technical documentation | Read-only plus document export |
| Replay | Reproduce a prior session against pinned state | Exact versions and evidence required |

### 10.4 Harness workflow UX

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ GONGCODE  Project: civil-api  Branch: feature/auth  Policy: GOV-RESTRICTED │
├─────────────────────────────────────────────────────────────────────────────┤
│ Task: Refactor JWT validation and add expired-token tests                   │
│                                                                             │
│ Proposed context                                                            │
│  ✓ src/auth/JwtProvider.java          allowed                              │
│  ✓ src/auth/AuthFilter.java           allowed                              │
│  ✓ tests/auth/*                       allowed                              │
│  ! config/application-prod.yml        metadata only; secrets removed       │
│                                                                             │
│ Proposed actions                                                            │
│  1. Read 8 files                                                            │
│  2. Modify 3 files                                                          │
│  3. Run ./gradlew test --tests "*Jwt*"                                      │
│  4. No network                                                              │
│                                                                             │
│ Risk: MEDIUM   Approval: not required until export                          │
│ [VIEW POLICY] [EDIT CONTEXT] [START]                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.5 Risk-tier behavior

- **Tier 0:** explanation, search, formatting. Automatic.
- **Tier 1:** ordinary code and tests inside non-sensitive paths. Automatic with logging.
- **Tier 2:** dependencies, database code, authentication, authorization, cryptography, network behavior, or infrastructure code. Additional checks and possible reviewer approval.
- **Tier 3:** production configuration, bulk data processing, schema migration, security policy, identity federation, public endpoint changes. Mandatory human approval.
- **Tier 4:** direct production changes, secret extraction, disabling controls, privileged infrastructure administration. Prohibited by default.

### 10.6 Harness local security

- No long-lived API keys in config files.
- Device-bound certificates stored through OS-approved secure storage.
- Signed binaries and extensions.
- Automatic binary integrity verification.
- No hidden background network calls.
- Clipboard controls configurable by classification.
- Screen capture restrictions delegated to endpoint security, with GongCode signaling sensitivity.
- Prompt and transcript local cache disabled or encrypted by default.
- Local cache has explicit TTL and secure deletion policy.
- Extension communicates only with the configured GongCode endpoint.
- User can inspect the exact endpoints and certificates in use.
- Developer mode is disabled in government production profiles.

## 11. GongCode Control: Unified Administration Platform

GongCode Control is the single operational console. Security, governance, provenance, model approval, GPU operations, evidence, and platform configuration are separate workspaces inside the same application, with strict role-based visibility.

### 11.1 Navigation

```text
Overview
Live Harnesses
Sessions
Security Operations
Prompt Governance
Policy Studio
Compliance Center
Provenance Explorer
Models
Evaluations
GPU & Capacity
Sandbox Fleet
Packages & Licenses
Identity & Access
Integrations
Evidence & Audit
Incidents
Reports
Platform Settings
```

### 11.2 Overview dashboard

The landing dashboard shows:

- connected harnesses,
- active users,
- active sessions,
- queued and running jobs,
- prompt and response token rates,
- model endpoint health,
- GPU utilization, VRAM, temperature, power, and queue depth,
- sandbox fleet health,
- policy allow/approval/block counts,
- open incidents,
- secret and PII blocks,
- prompt injection findings,
- package and license denials,
- evidence pipeline status,
- top agencies/projects by usage,
- SLO status,
- offline update status,
- certification-control drift.

Every metric supports drill-down. Administrators can filter by agency, tenant, project, classification, network zone, model, GPU pool, policy pack, severity, or time.

### 11.3 Live Harnesses

Each connected harness row shows:

- user and agency,
- managed device,
- repository and branch,
- task summary,
- session age,
- selected model,
- token rate,
- sandbox ID,
- files touched,
- active command,
- network state,
- risk score,
- policy profile,
- alert count,
- approval wait state.

Permitted actions:

- view sanitized live stream,
- request user confirmation,
- reduce capabilities,
- pause tool execution,
- isolate sandbox,
- terminate session,
- capture forensic snapshot,
- revoke session credentials,
- open incident.

Raw prompt access is not automatically granted to all administrators. Prompt viewing is permissioned, purpose-bound, logged, and may show only redacted content.

### 11.4 Session Inspector

Tabs:

1. **Timeline:** prompt, response, tool, decision, approval, scan, and export events.
2. **Context:** every context item, source, sensitivity, transformation, and reason.
3. **Files:** read/write spans and path decisions.
4. **Commands:** parsed command, executable, arguments, environment, result, and policy.
5. **Network:** attempted and permitted connections.
6. **Packages:** requested dependencies, mirror, version, SBOM, license, and vulnerability state.
7. **Policies:** rule-by-rule decisions and obligations.
8. **Provenance:** code spans and evidence links.
9. **Resources:** CPU, memory, disk, processes, token budget, and GPU allocation.
10. **Exports:** patches, reports, binaries, hashes, scans, and approvals.

### 11.5 Security Operations

Security Operations includes:

- real-time alert queue,
- severity and confidence,
- deterministic control that fired,
- related sessions, users, devices, repositories, and models,
- blast-radius graph,
- incident timeline,
- recommended containment,
- one-click isolation,
- evidence preservation,
- case assignment,
- status and SLA,
- root-cause notes,
- notification workflow,
- post-incident policy update.

Alert classes:

- credential access,
- secret disclosure,
- personal-information exposure,
- prompt injection,
- data exfiltration,
- unauthorized file access,
- unsafe command,
- sandbox escape indicator,
- policy tampering,
- audit interruption,
- unsigned update,
- model integrity failure,
- package supply-chain threat,
- prohibited license,
- cryptographic-policy violation,
- abnormal token or GPU usage,
- account anomaly,
- bulk repository read,
- protected branch write,
- suspicious archive or obfuscation,
- malicious test behavior.

### 11.6 Prompt Governance

Prompt Governance provides:

- prompt-template registry,
- approved system instructions,
- organization and project overlays,
- prompt classification,
- sensitive-term and data-category policies,
- retention profiles,
- redaction rules,
- model-specific templates,
- task-type templates,
- prompt version comparison,
- prompt evaluation results,
- prompt injection simulation,
- prompt and response sampling,
- access-controlled transcript review,
- legal hold,
- deletion and expiry reporting.

Raw prompts can be configured as:

- not stored,
- stored encrypted,
- stored redacted,
- stored as hash plus metadata,
- stored for a fixed period,
- stored only when an incident occurs,
- stored only with user notice and organizational authorization.

### 11.7 Policy Studio

Policy Studio has two layers:

1. **Simple controls:** checkboxes, selectors, thresholds, and approval matrices.
2. **Advanced policy-as-code:** versioned rules for security teams.

Example visible settings:

```text
[✓] Apply KISA software development security policy pack
[✓] Apply eGovFrame 5.0 engineering profile
[✓] Apply 개인정보 안전성 확보조치 evidence mapping
[✓] Require approved internal package mirrors
[✓] Prohibit hardcoded secrets
[✓] Prohibit MD5 for security-relevant use
[✓] Prohibit SHA-1 for new security-relevant use
[✓] Prohibit GPL-family dependencies
[ ] Permit LGPL dynamic-linking exception
[✓] Prohibit AGPL and SSPL
[✓] Prohibit System.out.println in production Java
[✓] Prohibit print() in production Python
[✓] Prohibit console.log in production JavaScript/TypeScript
[✓] Require structured logging
[✓] Require audit events for personal-data access
[✓] Require tests for AI-authored production code
[✓] Require SAST, dependency scan, secret scan, and SBOM
[✓] Require two reviewers for auth, crypto, and infrastructure code
[✓] Block direct push to protected branches
[✓] Block public internet from sandboxes
[✓] Require KCMVP-appropriate deployment profile where applicable
```

The UI must explain that some controls are organizational preferences, not legal requirements. “No GPL” and “no print” are examples of local coding and licensing policy rather than universal KISA requirements.

### 11.8 Compliance Center

Compliance Center provides:

- profile selection,
- current source/version metadata,
- control applicability,
- control owner,
- implementation status,
- evidence source,
- automated test status,
- exceptions,
- expiry,
- corrective action,
- audit report export.

Initial profiles:

- KISA/MOIS software development security guidance.
- Current eGovFrame engineering profile.
- CSAP service control mapping.
- ISMS-P control mapping.
- 개인정보 보호법 and current 개인정보의 안전성 확보조치 기준 mapping.
- Korean AI Basic Act risk and transparency workflow.
- KCMVP-aware cryptographic deployment profile.
- Organization-specific secure coding standard.
- OWASP ASVS and API Security profile.
- NIST SSDF and AI RMF optional cross-mapping.
- ISO/IEC 27001 and ISO/IEC 42001 optional cross-mapping.

The control engine must keep legal text, guidance, policy interpretation, product controls, and evidence requirements as separate versioned objects.

### 11.9 Provenance Explorer

Provenance Explorer allows an authorized user to click a line or block of code and answer:

- Was AI used?
- Which user initiated the task?
- Which human reviewers approved it?
- Which model and exact model artifact were used?
- What was the model approval status at that time?
- When was the code created or modified?
- Which session, commit, pull request, and repository state were involved?
- Which files, symbols, API schemas, documents, issues, and logs were used as context?
- Which tool calls and commands ran?
- Which policies allowed, transformed, required approval, or blocked actions?
- Which tests and scans passed or failed?
- Which dependencies and licenses were introduced?
- How much of the current code survives from the generated patch?
- Was the generated code later modified by a human or another model?
- Can the session be reproduced?
- What evidence bundle proves the history?

### 11.10 Model and GPU workspaces

Model screens cover:

- model inventory,
- approval state,
- model card,
- checksum and signature,
- license,
- source,
- architecture,
- tokenizer,
- context length,
- quantization,
- supported tasks,
- approved classifications,
- benchmark results,
- red-team results,
- expiration and reapproval,
- active endpoints,
- token usage,
- error rates,
- queue latency.

GPU screens cover:

- node and GPU health,
- utilization,
- VRAM,
- temperature,
- power,
- ECC and hardware errors,
- MIG partitions,
- model replicas,
- request queue,
- prefill/decode metrics,
- KV-cache use,
- throughput,
- time-to-first-token,
- tokens per second,
- concurrent sessions,
- admission controls,
- node drain and maintenance,
- capacity forecast.

### 11.11 Evidence and Audit

Evidence screens provide:

- immutable event search,
- evidence bundles by session, commit, release, project, policy, or incident,
- signed export,
- retention and legal hold,
- auditor access packages,
- control evidence freshness,
- missing evidence alerts,
- verification of hashes and signatures,
- replay manifests,
- certification-scope reports,
- access history for sensitive evidence.

---

## 12. GongCode Trace: AI Provenance and Explainability

### 12.1 Product behavior

GongCode Trace is both a platform service and a Control workspace. It records grounded engineering provenance rather than hidden reasoning.

For each code span, Trace records:

- repository and commit,
- parent commit,
- pull request,
- file path and language,
- AST symbol and semantic fingerprint,
- original and resulting text hashes,
- user,
- organization and project,
- task and session,
- model ID, version, weight hash, tokenizer hash, inference configuration, and endpoint,
- prompt template version and prompt hash,
- authorized prompt preview where retention permits,
- context items and transformations,
- tool calls,
- sandbox image and toolchain,
- packages and licenses,
- policy bundle and rule decisions,
- generated patch,
- human edits,
- model edits,
- review approvals,
- tests and scans,
- artifact export decision,
- evidence bundle,
- timestamps.

### 12.2 Code interaction design

```text
┌────────────────────────────────────────────────────────────────────────────┐
│ JwtProvider.java                                              AI 68%       │
├────────────────────────────────────────────────────────────────────────────┤
│ 81  public Claims validateToken(String token) {                           │
│ 82      Claims claims = parser.parseClaimsJws(token).getBody();           │
│ 83      verifyAudience(claims);                            [AI TRACE]       │
│ 84      verifyExpiration(claims);                          [AI TRACE]       │
│ 85      return claims;                                                     │
│ 86  }                                                                      │
└────────────────────────────────────────────────────────────────────────────┘
                         │ click lines 83–84
                         ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ PROVENANCE                                                                │
│ Commit       8fd21c7   PR #184                                            │
│ Created      2026-08-04 03:14:08 KST                                     │
│ Initiated by 김민수 / 행정안전부                                          │
│ Model        GC-Qwen-Coder-32B-GOV-v3 / weights sha256:...                │
│ Session      ses_01J... / sandbox sbx_a81d2c                              │
│ Context      JwtProvider.java, AuthFilter.java, auth spec v7, test logs   │
│ Tools        read 8 files, modified 3, ran 14 tests                       │
│ Policies     KISA pack v3.2, GOV-RESTRICTED v4.1                          │
│ Findings     0 critical, 0 high, 2 resolved medium                        │
│ Reviewers    박지현, 이성호                                               │
│ Survival     91% of original AI patch remains                             │
│ [WHY] [CONTEXT MAP] [TOOL TRACE] [TESTS] [POLICY] [REPLAY] [EVIDENCE]    │
└────────────────────────────────────────────────────────────────────────────┘
```

### 12.3 Span survival across refactors

Line numbers are insufficient because code moves. Trace uses:

- patch hunk mapping,
- AST node identifiers,
- normalized semantic fingerprints,
- token similarity,
- symbol lineage,
- Git rename detection,
- human-confirmed attribution for ambiguous merges.

Every merge recalculates surviving provenance. A block may have mixed attribution:

- AI generated,
- human edited,
- AI refactored,
- human reviewed,
- copied from approved template,
- generated from specification.

### 12.4 Explainability views

- **Why this code exists:** task, issue, requirement, and accepted proposal.
- **What influenced it:** context map and retrieved evidence.
- **What the model did:** generated diff and tool trace.
- **What humans changed:** before/after attribution.
- **How it was verified:** tests, scanners, review, and policy.
- **What remains uncertain:** missing tests, unsupported assumptions, or external API uncertainty.
- **What is prohibited:** do not expose fabricated chain-of-thought or hidden internal reasoning.

### 12.5 Provenance storage strategy

Store provenance outside Git as the source of truth, then publish references into:

- Git commit trailers,
- Git notes,
- pull-request checks,
- signed provenance manifests,
- optional `.gongtrace` sidecars for portable export.

A provenance manifest is signed and content-addressed. It references encrypted evidence rather than embedding sensitive prompts in Git.

---

## 13. Governance and Policy Architecture

### 13.1 Policy hierarchy

```text
Mandatory GongCode safety baseline
    ↓ may only be strengthened
Government-wide or central organization policy
    ↓
Agency policy
    ↓
Program policy
    ↓
Project policy
    ↓
Repository policy
    ↓
Task-specific temporary obligation
```

A lower layer cannot weaken a mandatory higher-layer rule. Every effective rule shows its source and precedence.

### 13.2 Rule categories

- identity and device,
- data classification,
- prompt handling,
- context access,
- personal information,
- secrets,
- file paths,
- code ownership,
- command execution,
- network,
- packages,
- licenses,
- cryptography,
- models,
- tokens and quotas,
- GPU pools,
- testing,
- code quality,
- approvals,
- branch protection,
- export,
- provenance,
- audit retention,
- incidents,
- offline updates.

### 13.3 Policy decisions

Supported results:

- allow,
- allow with transformation,
- allow with obligations,
- require user confirmation,
- require reviewer approval,
- require security approval,
- require two-person approval,
- quarantine,
- deny,
- terminate session,
- create incident.

### 13.4 Policy-as-code representation

Use an engine such as OPA/Rego for advanced rules, but expose a stable GongCode policy schema so the product is not coupled to one implementation.

Example organization policy:

```yaml
apiVersion: policy.gongcode.io/v1
kind: EngineeringPolicy
metadata:
  name: public-java-production
spec:
  appliesTo:
    languages: [java]
    environments: [production]
  rules:
    prohibitLicenses: [GPL-2.0-only, GPL-3.0-only, AGPL-3.0-only, SSPL-1.0]
    prohibitCrypto:
      securityUse: [MD5, SHA-1, DES, RC4]
    prohibitPatterns:
      - id: no-system-out
        matcher: java.ast.methodCall
        value: java.lang.System.out.println
    require:
      - structuredLogging
      - unitTests
      - sast
      - dependencyScan
      - secretScan
      - sbom
    approvals:
      authenticationChanges: 2
      cryptographyChanges: 2
      infrastructureChanges: 2
```

### 13.5 Policy lifecycle

1. Draft.
2. Automated syntax and conflict validation.
3. Simulation against historical events.
4. Impact report.
5. Security review.
6. Approval.
7. Signed publication.
8. Staged rollout.
9. Enforcement.
10. Drift monitoring.
11. Expiration or supersession.
12. Evidence retention.

### 13.6 Exception lifecycle

Every exception includes:

- rule,
- requested scope,
- business reason,
- compensating controls,
- requester,
- approver,
- start and expiry,
- affected repositories,
- review date,
- usage count,
- linked incidents,
- automatic revocation.

Permanent undocumented bypasses are not supported.

---

## 14. Compliance and Korean Public-Sector Baselines

### 14.1 Source baselines as of the plan date

The initial product backlog should map to current official sources, including:

- eGovFrame 5.0 development and runtime guidance.
- MOIS/KISA software development security and software weakness diagnostic guidance.
- `행정기관 및 공공기관 정보시스템 구축·운영 지침`, current published version.
- CSAP certification types, grades, and control sets.
- ISMS-P management, protection, and privacy control domains.
- `개인정보의 안전성 확보조치 기준`, PIPC Notice No. 2026-9, effective 2026-07-01.
- `인공지능 발전과 신뢰 기반 조성 등에 관한 기본법` and its enforcement decree, effective 2026-07-21.
- KCMVP program requirements where cryptographic-module procurement rules apply.

All source documents must be ingested into a controlled reference registry with publisher, title, issue date, effective date, superseded status, checksum, reviewer, and internal interpretation notes.

### 14.2 Compliance profile behavior

Each profile contains:

- official control or guidance reference,
- plain-language interpretation,
- applicability conditions,
- GongCode preventive control,
- GongCode detective control,
- required organizational process,
- evidence artifacts,
- automated test,
- manual evidence,
- responsible role,
- review frequency,
- exceptions,
- version history.

### 14.3 Important distinction

The platform must distinguish:

- **Law or binding rule**
- **Certification criterion**
- **Government guidance**
- **Secure coding recommendation**
- **Organization policy**
- **Project convention**

For example, “no GPL” and “no `print()`” are configurable organization rules. They must not be presented as requirements of KISA or CSAP unless an official source specifically establishes that obligation for the applicable scope.

### 14.4 Initial mapping outcomes

The initial compliance release should be able to generate:

- CSAP-oriented service scope and asset inventory.
- ISMS-P-oriented control evidence matrix.
- Personal-information access, encryption, logging, retention, and incident evidence.
- Software development security findings and remediation evidence.
- AI model inventory and approval register.
- AI risk assessment and impact-assessment workflow.
- Access-right grant/change/revocation history.
- Prompt and personal-data retention report.
- Cryptographic module and algorithm inventory.
- Secure development lifecycle evidence.
- Incident response and forensic evidence.
- Backup, restore, and DR test evidence.
- Change and release-management evidence.

## 15. Model Platform, Approval, and Korean Government Intelligence

### 15.1 GongCode Gateway

Gateway responsibilities:

- model endpoint discovery,
- classification-aware routing,
- prompt-template injection,
- request and response scanning,
- token accounting,
- per-user, project, and agency quotas,
- concurrency and admission control,
- timeout and retry policy,
- context-length enforcement,
- deterministic model fallback rules,
- model approval enforcement,
- inference parameter policy,
- request correlation and provenance,
- cache policy,
- endpoint health and circuit breaking.

The sandbox calls only Gateway. It never calls vLLM, SGLang, CUDA, or a GPU node directly.

### 15.2 Model Registry

Registry objects:

- base model,
- fine-tune or adapter,
- quantized artifact,
- tokenizer,
- chat template,
- serving container,
- inference engine,
- evaluation suite,
- model card,
- license and use restrictions,
- source and chain of custody,
- checksums and signatures,
- approved deployment profiles,
- known limitations,
- expiry and reapproval date.

### 15.3 Model approval workflow

1. Intake request with use case and source.
2. Artifact hash and signature verification.
3. Malware and unsafe serialization scan.
4. License and redistribution review.
5. Architecture and tokenizer inspection.
6. Training-data provenance statement review where available.
7. Offline functionality and network behavior verification.
8. Korean language and government coding benchmark.
9. Secure coding benchmark.
10. Hallucination and unsupported-API benchmark.
11. Secret-exfiltration and memorization tests.
12. Prompt-injection robustness tests.
13. Tool-use safety tests.
14. Long-context retrieval tests.
15. eGovFrame, Spring, MyBatis, SQL, batch, and legacy-code tests.
16. Performance and GPU-capacity test.
17. Classification-specific risk assessment.
18. Human approval with scope and expiry.
19. Signed model deployment.
20. Continuous regression and incident-triggered reapproval.

Approval states:

- submitted,
- quarantined,
- evaluating,
- conditionally approved,
- approved,
- restricted,
- suspended,
- expired,
- rejected,
- retired.

### 15.4 Fine-tune versus retrieval versus policy

Use each mechanism for the right purpose:

| Knowledge or behavior | Primary mechanism |
|---|---|
| Stable Korean coding explanations and interaction style | SFT |
| Secure engineering behavior and tool discipline | SFT plus agent policy |
| eGovFrame code patterns and common components | SFT plus code retrieval |
| Current eGovFrame version documentation | Versioned RAG |
| Current laws, notices, and CSAP/ISMS-P criteria | Versioned RAG and policy packs |
| Prohibited licenses or algorithms | Deterministic policy engine |
| Organization coding conventions | Policy packs and repository retrieval |
| Agency architecture templates | Retrieval and signed templates |
| High-risk approval requirements | Deterministic policy engine |
| Model refusal and safe escalation behavior | SFT/evaluation, reinforced by external controls |

Do not encode mutable compliance text only into model weights. The model should cite the exact controlled source version used for compliance-related explanations.

### 15.5 Training data domains

Subject to license and authority:

- eGovFrame 5.0 documentation, examples, templates, and public code.
- Spring and Spring Security official documentation and approved examples.
- MyBatis, JPA/Hibernate, Java, Kotlin, Python, JavaScript/TypeScript, C/C++, and SQL secure patterns.
- MOIS/KISA software development security guidance and public examples.
- Korean public-sector API patterns and public data API documentation.
- Publicly available government software architecture and data-standard materials.
- Korean technical writing and code-review language.
- Publicly releasable legacy modernization examples.
- Approved internal synthetic datasets.
- Vulnerable-to-secure code transformations.
- Security review rationales grounded in published rules.
- Tests, build files, CI pipelines, infrastructure-as-code, SBOMs, and release evidence.

### 15.6 Data curation rules

- Verify copyright and license for every source.
- Separate training, evaluation, and red-team sets.
- Deduplicate at repository, file, function, and semantic levels.
- Remove real credentials and personal information.
- Replace government-sensitive examples with synthetic equivalents.
- Tag framework, version, language, vulnerability class, and policy source.
- Track source document effective and superseded dates.
- Retain data lineage and transformation history.
- Prevent customer code from entering future training unless an explicit separate agreement and approved pipeline exist.
- Make “no customer training” the default and visible policy.

### 15.7 Evaluation suites

Create named evaluation packs:

- `GC-KO-CODE`: Korean coding instruction understanding.
- `GC-EGOV-5`: eGovFrame 5.0 implementation and migration.
- `GC-KISA-SDLC`: software development security.
- `GC-PUBLIC-JAVA`: Java/Spring/MyBatis public-sector applications.
- `GC-LEGACY`: JSP, XML configuration, batch, and monolith modernization.
- `GC-SECURE-TOOL`: tool-use permission discipline.
- `GC-INJECTION`: repository and document prompt injection.
- `GC-PII-KR`: Korean personal-information detection and minimization.
- `GC-PROVENANCE`: source and evidence attribution.
- `GC-LONG-CONTEXT`: large repository and 256K-class context retrieval.
- `GC-APPLICABILITY`: distinguish legal rule, guidance, and organization preference.
- `GC-HALLUCINATION`: unsupported API, class, method, and configuration claims.
- `GC-EXPLAIN`: grounded code explanation without fabricated reasoning.

Every production model version must have pass thresholds by intended classification and task.

---

## 16. GPU and Inference Infrastructure

### 16.1 Design objectives

- Maximize concurrent coding sessions before maximizing single-request peak tokens per second.
- Support long repository contexts while preventing one session from monopolizing capacity.
- Keep GPU administration isolated from developers and sandboxes.
- Provide deterministic admission control and priority for critical agencies.
- Allow dedicated, pooled, and partitioned GPU deployments.
- Support approved open models and future inference engines.

### 16.2 Reference topology

```text
Harness / internal client
          │
          ▼
GongCode Gateway
  ├─ Identity and quota
  ├─ Prompt and response controls
  ├─ Model approval
  ├─ Routing and admission
  └─ Token/provenance events
          │ mTLS
          ▼
Inference control network
  ├─ vLLM endpoint pool
  ├─ SGLang endpoint pool
  ├─ embedding endpoints
  ├─ reranker endpoints
  └─ safety/classifier endpoints
          │
          ▼
Dedicated GPU worker network
  ├─ H100/H200 high-context pool
  ├─ L40S/A100 balanced pool
  ├─ MIG partitions where approved
  └─ evaluation and canary pool
```

### 16.3 GPU node controls

- Separate GPU management network.
- No user SSH from development zones.
- Immutable or tightly managed node images.
- Signed model containers and weight artifacts.
- No direct repository mounts.
- No sandbox credentials.
- Restricted NVIDIA management capabilities.
- GPU telemetry exported without prompt content.
- Node attestation before receiving model artifacts.
- Per-endpoint model hash verification.
- GPU memory cleared or process-isolated between incompatible workloads as supported.
- Dedicated nodes for the highest classifications.
- MIG only where the agency accepts the isolation profile.
- Maintenance and drain workflow with approval and evidence.
- Hardware error, ECC, thermal, and power alerts.

### 16.4 Serving features

- Continuous batching.
- Prefix caching where policy permits cross-request reuse.
- Tenant-safe cache partitioning.
- Paged KV cache.
- Chunked prefill.
- Classification-aware maximum context.
- Context compression only when approved and provenance-preserving.
- Speculative decoding only after model-pair approval.
- Quantized models only as separately approved artifacts.
- Request cancellation and budget enforcement.
- Per-model concurrency caps.
- Deadline and priority queues.
- Separate prefill/decode pools where justified by scale.
- Autoscaling within the closed cluster.
- Capacity reservation for incident response and critical projects.
- Canary endpoints for new versions.
- Deterministic rollback.

### 16.5 Metrics

- requests per second,
- concurrent requests,
- queued requests,
- time to first token,
- inter-token latency,
- input and output tokens per second,
- prefill and decode utilization,
- KV cache utilization and eviction,
- prefix-cache hit rate,
- GPU utilization and VRAM,
- model replica health,
- request cancellation,
- OOM and restart count,
- classification and tenant distribution,
- token cost allocation,
- approval and policy failure rates.

### 16.6 Initial capacity profiles

These are planning profiles, not promises:

| Profile | Example hardware | Intended use |
|---|---|---|
| Evaluation | 1–2 data-center GPUs | Model tests, small pilot, no HA |
| Department | 4 GPUs across 2 nodes | Moderate coding concurrency and redundancy |
| Agency | 8–16 GPUs across 4+ nodes | Multiple models, HA, long context, evaluation pool |
| Central shared service | 32+ GPUs | Multi-agency capacity with strict tenancy and dedicated high-classification pools |
| Air-gapped dedicated | Customer-selected fixed fleet | Predictable workloads and offline operations |

Capacity must be benchmarked with the exact approved model, quantization, context profile, concurrency, and workload. Marketing must not infer concurrency from parameter count alone.

### 16.7 Embedding and retrieval infrastructure

- Separate approved embedding and reranking models.
- Per-project or per-classification vector collections.
- Metadata and access filtering before vector search.
- Document chunk provenance.
- Deletion and reindexing workflows.
- No cross-tenant retrieval.
- Optional keyword and Korean morphological search.
- Index versioning and rollback.
- Retrieval quality and leakage tests.

---

## 17. GongCode Runtime and Sandbox

### 17.1 Baseline boundary

For protected government code, the baseline is:

```text
Hardened bare-metal or virtualization host
    └─ KVM-backed microVM per session
         └─ rootless OCI container
              └─ GongCode tool runner
                   └─ nested restricted process sandbox for generated binaries
```

A standard shared Docker container is not the primary isolation boundary for restricted workloads.

### 17.2 Sandbox lifecycle

1. Receive signed sandbox specification.
2. Verify user, project, repository, image, and policy.
3. Allocate host and resource envelope.
4. Boot microVM from immutable signed image.
5. Attach ephemeral encrypted workspace.
6. Import repository snapshot through Git broker.
7. Mount approved toolchains and package cache read-only.
8. Start audit and runtime agents.
9. Confirm network is denied by default.
10. Execute approved actions.
11. Scan candidate exports.
12. Create provenance and evidence.
13. Optionally capture forensic snapshot on incident.
14. Destroy VM and discard workspace encryption key.
15. Record destruction evidence.

### 17.3 Filesystem

```text
/workspace   writable repository snapshot
/toolchain   read-only approved compilers and tools
/packages    read-only or brokered approved cache
/policy      read-only effective policies and task manifest
/input       read-only approved imported artifacts
/output      only exportable location
/tmp         ephemeral, size-limited, noexec where compatible
/run/gong    agent sockets with narrow permissions
/secrets     absent by default; process-scoped injection only
```

Never mount:

- developer home directory,
- host `/proc` or `/sys`,
- Docker or containerd socket,
- Kubernetes service account,
- host SSH agent,
- cloud metadata credentials,
- broad NFS shares,
- production kubeconfig,
- GPU management sockets.

### 17.4 Runtime restrictions

- non-root user,
- no `sudo`,
- no privileged container,
- minimal Linux capabilities,
- deny-by-default seccomp,
- SELinux/AppArmor profile,
- cgroup CPU, memory, IO, PID, and device controls,
- process and file descriptor limits,
- maximum file and archive size,
- no kernel module loading,
- no arbitrary mount,
- no raw sockets,
- no host namespace access,
- controlled `ptrace`,
- restricted BPF and performance events,
- execution and inactivity timeouts.

### 17.5 Network

Default:

```text
Sandbox → no route
```

Approved route:

```text
Sandbox
  └─ Network Box / authenticated proxy
       ├─ Git broker
       ├─ package mirror
       ├─ artifact repository
       ├─ approved test service
       ├─ internal documentation retrieval
       └─ GongCode Gateway
```

Destination policy includes DNS name, resolved IP, certificate identity, port, protocol, direction, request method, content type, purpose, rate, and maximum transferred bytes.

### 17.6 Package installation

The sandbox cannot install arbitrary public dependencies. Flow:

1. Agent requests package and exact version.
2. Package Box checks approved mirrors.
3. License Box evaluates SPDX policy.
4. Vulnerability and malware scans run.
5. Package provenance and checksum are verified.
6. Approval is requested when policy requires.
7. Package is delivered through a scoped, expiring mirror token.
8. SBOM and provenance are updated.

### 17.7 Secret brokering

- Secrets are never included in prompts.
- Long-lived production secrets are not provided to autonomous sessions.
- Integration tests use dedicated low-privilege identities.
- Secret broker issues process-scoped, target-scoped, short-lived credentials.
- Values are injected without writing to workspace where possible.
- Commands and output are redacted.
- Credential use is independently logged.
- Tokens are revoked when the command exits or session ends.
- Break-glass secret use creates an incident and mandatory review.

### 17.8 GPU test sandbox

Generated CUDA or ML code requiring GPU executes in a separate approved pool:

- dedicated VM or approved partition,
- no model-serving weights or control credentials,
- explicit GPU device allocation,
- strict job time and memory limits,
- separate network policy,
- no shared home directory,
- GPU reset/cleanup procedure,
- additional approval for high-classification code.

### 17.9 Windows support

A separate Windows sandbox pool may be added for .NET, PowerShell, or Windows-specific public systems:

- Hyper-V or approved VM isolation,
- ephemeral VM per session,
- constrained PowerShell/JEA patterns,
- application control,
- signed tools,
- separate image lifecycle,
- no interoperability shortcut that weakens Linux sandbox controls.

### 17.10 Browser and external content

Browser activity is not performed inside the code sandbox. A separate retrieval service:

- fetches only approved destinations,
- strips active content,
- scans downloads,
- labels external instructions as untrusted,
- returns sanitized text or artifacts,
- records source and retrieval time,
- never exposes browser cookies to the model or sandbox.

---

## 18. Security Architecture and Threat Model

### 18.1 Core threats

| Threat | Example | Primary controls |
|---|---|---|
| Prompt injection | README instructs agent to exfiltrate secrets | Injection Box, capability controls, no network |
| Malicious developer | User asks agent to read unauthorized repository | Identity, Context, File Boxes |
| Compromised model | Model requests host or production access | Tool broker and deterministic policy |
| Malicious dependency | Package install script steals data | Package mirror, sandbox, no network, scans |
| Secret leakage | `.env` enters prompt or logs | Secrets Box, Output Box, redaction |
| PII leakage | Resident number appears in logs/context | PII Box, purpose and minimization policy |
| Sandbox escape | Generated binary exploits runtime | MicroVM, nested sandbox, host monitoring |
| Cross-tenant access | Retrieval returns another agency’s code | Tenant keys, metadata filters, ABAC |
| Model poisoning | Modified weights behave maliciously | Registry hash, signature, approval, canary |
| Audit tampering | Session deletes or rewrites logs | External append-only evidence plane |
| Admin compromise | Admin disables controls | Separation of duties, two-person approval |
| Update compromise | Offline bundle includes altered binary | Signed bundle, SBOM, import review |
| GPU side channel | Tenant infers another workload | dedicated pools, approved partitioning |
| Data exfiltration | Archive uploaded to approved-looking host | content-aware egress, byte limits, DLP |
| Excessive collection | Entire repository sent unnecessarily | context minimization and budget |
| Provenance forgery | Commit claims false AI history | signed events and content-addressed manifests |
| Public website compromise | Marketing site becomes bridge to product | complete network and identity separation |

### 18.2 Zero-trust controls

- Authenticate and authorize every user, device, service, and workload.
- mTLS for service-to-service traffic.
- Short-lived workload identities.
- No implicit trust from network location.
- Explicit policy per action.
- Continuous device and session risk.
- Network segmentation enforced outside workloads.
- Separate management, control, execution, model, evidence, and supply-chain networks.
- Privileged operations through hardened administration paths.
- Administrative actions recorded and reviewed.

### 18.3 Encryption

- Encryption in transit with approved TLS profile.
- Encryption at rest with customer-controlled keys where required.
- Per-tenant and per-evidence-domain key separation.
- HSM or approved key-management integration.
- Cryptographic inventory with algorithm, key size, module, purpose, owner, and expiry.
- Crypto Box prevents new use of prohibited algorithms.
- KCMVP-validated modules used where applicable to the procurement and deployment scope.
- Key rotation, escrow, recovery, revocation, and destruction evidence.

### 18.4 Personal information controls

PII Box must support Korean identifiers and patterns, including:

- resident registration number,
- foreigner registration number,
- passport number,
- driver’s license number,
- phone number,
- email,
- address,
- account and card numbers,
- health and biometric information,
- employee and civil-service identifiers,
- combinations that identify an individual.

Detection combines deterministic patterns, validation rules, dictionaries, context classification, and approved local ML classifiers. It must support:

- block,
- mask,
- tokenize,
- pseudonymize,
- minimize fields,
- require purpose,
- require approval,
- prevent retention,
- alert and create incident.

False positives and false negatives are measured against Korean datasets.

### 18.5 Prompt-injection controls

- Treat repository, issue, document, website, log, and test text as data, not authority.
- Label content source and trust level.
- Separate system policy from retrieved content.
- Prevent retrieved content from changing tool capabilities.
- Strip or mark instruction-like passages.
- Detect common and obfuscated injection patterns.
- Limit tool-call argument construction.
- Require approval for risk escalation.
- Run adversarial regression suites.
- Record injection findings in provenance.

### 18.6 Software supply chain

- Signed source releases.
- Signed container and sandbox images.
- SBOM for platform and exported artifacts.
- Internal package mirrors.
- Dependency pinning and checksum verification.
- License policy.
- Vulnerability scanning.
- SLSA-style provenance where feasible.
- Reproducible build targets for critical components.
- Offline signature verification.
- Build service separated from production signing service.
- Two-person release approval.
- Rapid revocation and rollback.

### 18.7 Host and runtime detection

Monitor outside the sandbox for:

- unusual system calls,
- namespace or mount attempts,
- raw socket creation,
- port scanning,
- mass file reads,
- high-entropy archives,
- obfuscation,
- credential discovery,
- crypto-mining behavior,
- unexpected process trees,
- kernel exploit indicators,
- excessive logs,
- outbound transfer anomalies,
- audit-agent interruption.

### 18.8 Fail-safe behavior

Protected operations stop when any mandatory service is unavailable:

- policy engine unavailable,
- audit sink unavailable beyond local signed buffer threshold,
- model approval unavailable,
- secrets or PII scanner unavailable for protected data,
- sandbox attestation failed,
- evidence signature failed,
- update signature invalid.

Read-only explanatory access may continue under a separately defined degraded-mode policy, with a visible warning and no export.

## 19. Data Architecture

### 19.1 Primary data domains

| Domain | Data | Preferred store |
|---|---|---|
| Control | organizations, users, projects, sessions, policies, approvals | PostgreSQL-compatible relational database |
| Events | prompt, tool, policy, runtime, model, security, and admin events | Apache Kafka-compatible event bus |
| Analytics | token, session, GPU, policy, and fleet metrics | ClickHouse or equivalent columnar store |
| Search | alerts, audit metadata, documentation, event text | OpenSearch or equivalent |
| Evidence | signed bundles, scans, reports, snapshots, manifests | Approved S3-compatible object store with retention controls |
| Provenance | code spans, graphs, manifests, commit mappings | Relational store plus graph/index services |
| Secrets | platform secrets and short-lived issuance state | OpenBao or approved enterprise secret manager |
| Policies | signed bundles and history | Git-backed source plus registry |
| Models | model metadata, approvals, evaluations | Registry database plus artifact store |
| Telemetry | metrics, traces, logs | OpenTelemetry pipeline and approved backends |
| Retrieval | document metadata, lexical index, vector index | Approved search/vector services with tenant isolation |

The final deployment may replace individual technologies, but public APIs and data contracts must remain stable.

### 19.2 Core entities

- `Organization`
- `Agency`
- `Tenant`
- `User`
- `Role`
- `Device`
- `Project`
- `Repository`
- `BranchProtection`
- `Session`
- `Task`
- `PromptExchange`
- `ContextItem`
- `ToolCall`
- `AssuranceDecision`
- `ApprovalRequest`
- `Sandbox`
- `RuntimeEvent`
- `Model`
- `ModelArtifact`
- `ModelApproval`
- `EvaluationRun`
- `GPUNode`
- `GPUPartition`
- `InferenceEndpoint`
- `Package`
- `LicenseDecision`
- `SecurityFinding`
- `Incident`
- `ProvenanceSpan`
- `EvidenceBundle`
- `ComplianceProfile`
- `ComplianceControl`
- `Exception`
- `OfflineUpdateBundle`

### 19.3 Data classification

Every object carries:

- organization and tenant,
- project,
- classification,
- data categories,
- owner,
- retention profile,
- legal hold,
- allowed regions/zones,
- encryption key reference,
- access-control labels,
- source and provenance,
- deletion state.

### 19.4 Retention

Retention is defined independently for:

- raw prompt,
- redacted prompt,
- model response,
- tool results,
- source snapshots,
- runtime logs,
- policy decisions,
- security events,
- code provenance,
- evidence bundles,
- forensic snapshots,
- user analytics.

Retention policies must support the current legal and organizational requirements for access logs, while avoiding indefinite retention of sensitive prompt content. Deletion must propagate to caches and indexes while preserving required non-content evidence such as hashes, decision metadata, and legal-hold records.

---

## 20. Service APIs and Event Contracts

### 20.1 External APIs

- Session API
- Harness configuration API
- Repository onboarding API
- Policy evaluation API
- Approval API
- Model Gateway API
- Provenance query API
- Evidence export API
- Security incident API
- Model Registry API
- Evaluation API
- GPU capacity API
- Offline update API
- Audit verification API

### 20.2 Initial endpoint families

```text
POST   /v1/sessions
GET    /v1/sessions/{id}
POST   /v1/sessions/{id}/actions
POST   /v1/policies/evaluate
POST   /v1/approvals
POST   /v1/gateway/chat
POST   /v1/gateway/embeddings
GET    /v1/provenance/repos/{repo}/commits/{sha}
GET    /v1/provenance/spans/{spanId}
POST   /v1/evidence/bundles
GET    /v1/models
POST   /v1/models/{id}/approval
POST   /v1/evaluations
GET    /v1/gpu/capacity
POST   /v1/incidents/{id}/contain
POST   /v1/updates/import
POST   /v1/audit/verify
```

### 20.3 Event topics

```text
gongcode.session.lifecycle
gongcode.prompt.exchange
gongcode.context.decision
gongcode.tool.request
gongcode.assurance.decision
gongcode.runtime.event
gongcode.model.request
gongcode.gpu.telemetry
gongcode.security.finding
gongcode.incident.lifecycle
gongcode.provenance.span
gongcode.evidence.bundle
gongcode.policy.lifecycle
gongcode.model.lifecycle
gongcode.update.lifecycle
gongcode.admin.action
```

Events are versioned, schema-validated, signed where required, and correlated through organization, project, session, action, and trace IDs.

---

## 21. Integrations

### 21.1 Source control

Initial:

- GitLab Self-Managed
- GitHub Enterprise Server
- Gitea/Forgejo where approved

Capabilities:

- repository snapshot,
- branch protection,
- pull/merge request,
- status checks,
- signed commit candidate,
- reviewers,
- Git notes,
- provenance links,
- webhook or polling for closed networks.

### 21.2 CI/CD

- Jenkins
- GitLab CI
- GitHub Enterprise Actions where deployed
- Tekton or Argo Workflows where approved

GongCode does not replace CI/CD. It emits signed artifacts, checks, evidence, and policy decisions that CI consumes.

### 21.3 Package and artifact repositories

- Nexus Repository
- JFrog Artifactory
- Harbor for approved container workflows
- approved language-specific mirrors

Integrations must expose package provenance, license, vulnerability, signature, and approval state.

### 21.4 Identity

- SAML 2.0
- OIDC
- LDAP/Active Directory
- government or agency PKI integration
- smart card or hardware-token MFA
- device certificate
- SCIM where available

### 21.5 Knowledge and documents

- internal document management systems,
- approved wiki,
- issue tracker,
- API catalog,
- architecture repository,
- controlled regulations/guidance corpus.

All retrieval is permission-filtered before search and before model context.

### 21.6 Security operations

- SIEM
- SOAR
- endpoint security
- vulnerability management
- ticketing/case management
- time synchronization
- HSM/KMS
- malware scanning
- network detection

### 21.7 Government development ecosystem

- eGovFrame project metadata and templates,
- approved Java and application-server toolchains,
- Oracle/PostgreSQL/MariaDB testing,
- Korean encoding and document formats,
- public-data API catalogs,
- Korean accessibility test tooling,
- public-sector standard terminology and error formats.

---

## 22. Deployment Editions

### 22.1 GongCode Sovereign

For air-gapped or tightly closed networks:

- fully on-premises,
- customer-owned GPU,
- local identity,
- local model registry,
- local package/document mirrors,
- signed offline updates,
- no required external telemetry,
- optional vendor support through controlled evidence export.

### 22.2 GongCode Private

For private cloud or government data center:

- multi-project private service,
- private GPU pool,
- HA control plane,
- managed integrations,
- restricted update gateway,
- central operations.

### 22.3 GongCode CSAP Cloud

A separately scoped SaaS deployment intended for formal CSAP assessment:

- public-sector cloud architecture,
- certification-scope asset inventory,
- tenant isolation,
- control evidence,
- vulnerability and penetration-test process,
- operations and support procedures,
- annual follow-up and renewal readiness.

This edition cannot inherit certification merely from the on-premises product. Certification scope, infrastructure, procedures, personnel, and evidence must be assessed.

### 22.4 GongCode Hybrid

- Control and evidence remain in customer environment.
- Selected model workloads use an approved private shared GPU zone, or the reverse.
- Cross-zone requests are minimized, encrypted, classified, and brokered.
- No hybrid path is enabled for data classes that prohibit it.
- Provenance records the exact execution location and model endpoint.

### 22.5 GongCode SI

For system integrators delivering government projects:

- multi-customer deployment templates,
- customer-separated policy packs,
- evidence handoff,
- implementation toolkit,
- standardized onboarding,
- white-label prohibited unless contractually approved,
- complete attribution and subcontractor access records.

---

## 23. Reference Technical Stack

This is a recommended starting point and must pass license, security, and deployment review.

### 23.1 Application

- **Backend services:** Go for control and high-concurrency services; Rust for endpoint/runtime/security agents where memory safety is valuable.
- **Web applications:** TypeScript, React, Next.js.
- **CLI:** Rust or Go, statically linked where practical.
- **IDE:** TypeScript for VS Code; Java for Eclipse integration.
- **Contracts:** Protobuf and JSON Schema.
- **Authentication:** Keycloak or approved identity provider.
- **Policy:** OPA-compatible engine behind GongCode policy API.
- **Secrets:** OpenBao or approved enterprise secret manager.
- **Observability:** OpenTelemetry.

### 23.2 Platform

- Kubernetes for control, model, and service orchestration.
- KVM-backed Firecracker, Kata Containers, or equivalent approved microVM layer.
- Cilium or equivalent for externally enforced network policy.
- PostgreSQL.
- Apache Kafka.
- ClickHouse.
- OpenSearch.
- Approved object storage.
- vLLM and SGLang as initial inference engines.
- Sigstore-compatible internal signing and verification.
- Syft/Grype, Trivy, Semgrep, CodeQL-equivalent, language scanners, and commercial scanners as approved.

### 23.3 License discipline

The platform dependency baseline should prefer permissive licenses and maintain an explicit software bill of materials. If a deployment enables a “no GPL-family” policy, the product build and bundled components for that edition must be validated against it; merely blocking generated-code dependencies is insufficient.

---

## 24. Proposed Monorepo Structure

```text
gongcode/
├── apps/
│   ├── harness-cli/
│   ├── harness-web/
│   ├── control-web/
│   ├── public-web/
│   ├── vscode-extension/
│   └── eclipse-plugin/
├── services/
│   ├── session/
│   ├── orchestrator/
│   ├── identity-authorizer/
│   ├── device-posture/
│   ├── prompt-governor/
│   ├── context-firewall/
│   ├── secret-sentinel/
│   ├── pii-shield/
│   ├── injection-defender/
│   ├── file-guard/
│   ├── command-guard/
│   ├── network-gate/
│   ├── package-gate/
│   ├── license-guard/
│   ├── crypto-guard/
│   ├── model-authorizer/
│   ├── runtime-warden/
│   ├── artifact-gate/
│   ├── response-inspector/
│   ├── resource-governor/
│   ├── trace-writer/
│   ├── evidence-builder/
│   ├── audit-forwarder/
│   ├── model-gateway/
│   ├── model-registry/
│   ├── evaluation/
│   ├── gpu-control/
│   ├── incident/
│   └── update-verifier/
├── agents/
│   ├── sandbox-agent/
│   ├── host-runtime-agent/
│   ├── gpu-node-agent/
│   └── offline-update-agent/
├── packages/
│   ├── contracts/
│   ├── policy-sdk/
│   ├── provenance-sdk/
│   ├── evidence-sdk/
│   ├── ui-system/
│   ├── korean-pii/
│   └── secure-code-rules/
├── policies/
│   ├── baseline/
│   ├── kisa/
│   ├── egovframe/
│   ├── csap/
│   ├── isms-p/
│   ├── privacy/
│   ├── ai-basic-act/
│   └── examples/
├── evaluations/
│   ├── gc-ko-code/
│   ├── gc-egov-5/
│   ├── gc-kisa-sdlc/
│   ├── gc-injection/
│   └── gc-pii-kr/
├── deploy/
│   ├── helm/
│   ├── operators/
│   ├── airgap/
│   ├── csap-cloud/
│   └── reference-architectures/
├── images/
│   ├── sandbox/
│   ├── model-serving/
│   └── offline-bundles/
├── docs/
│   ├── architecture/
│   ├── controls/
│   ├── operations/
│   ├── threat-model/
│   ├── api/
│   └── website/
└── tests/
    ├── integration/
    ├── security/
    ├── conformance/
    ├── chaos/
    └── end-to-end/
```

Repositories may later split by security boundary, but contracts and release provenance must remain coordinated.

---

## 25. Reliability, High Availability, and Disaster Recovery

### 25.1 Availability targets

Initial production targets:

- Control API: 99.9% monthly.
- Model Gateway: 99.9% excluding approved maintenance.
- Policy evaluation: 99.99% within a deployment cluster.
- Audit event durability: no acknowledged protected action without durable signed buffering.
- Evidence bundle creation: 99.9%, with backlog visibility.
- Sandbox launch success: 99.5%.
- Public website: 99.9%, completely independent from product availability.

### 25.2 Recovery objectives

Set deployment-specific RPO/RTO, with suggested starting points:

| System | RPO | RTO |
|---|---:|---:|
| Policies and approvals | 0–5 minutes | 1 hour |
| Audit and evidence | near zero | 4 hours |
| Control metadata | 5 minutes | 2 hours |
| Provenance | 5 minutes | 4 hours |
| Model registry | 15 minutes | 4 hours |
| Analytics | 1 hour | 24 hours |
| Public website | source-controlled | 2 hours |

### 25.3 DR tests

- quarterly restore tests,
- annual region/site failover where applicable,
- evidence-signature verification after restore,
- model artifact integrity checks,
- policy bundle consistency checks,
- identity dependency failure exercises,
- audit pipeline outage exercise,
- GPU fleet capacity-loss exercise,
- offline backup media verification.

---

## 26. Responsible AI Program

### 26.1 Governance lifecycle

- use-case registration,
- risk classification,
- model selection,
- impact assessment,
- evaluation,
- approval,
- deployment,
- monitoring,
- incident management,
- periodic review,
- retirement.

### 26.2 Coding-agent risk controls

- AI does not have independent authority.
- AI-generated code is labeled.
- High-risk code requires human review.
- Tests and security scans are mandatory by policy.
- Unsupported claims are surfaced.
- Model limitations are visible.
- User can inspect context and tool history.
- Users receive training on automation bias.
- Acceptance-rate metrics are not used alone as employee-performance metrics.
- Monitoring should avoid unnecessary surveillance of developer content.
- Governance reports separate security outcomes from productivity analytics.

### 26.3 Model and agent incident types

- unsafe code generation,
- data leakage,
- systematic hallucination,
- policy evasion,
- prompt injection success,
- unauthorized tool request,
- anomalous refusal or bias,
- model artifact integrity failure,
- excessive resource use,
- provenance gap.

### 26.4 Human factors

- clear distinction between suggestion and executed action,
- visible permission prompts,
- understandable rule explanations,
- no dark patterns encouraging approval,
- high-risk approval cannot be bundled with low-risk actions,
- reviewer sees tests and provenance before approving,
- accessible Korean-language alerts,
- reduced-motion and keyboard-accessible interfaces.

---

## 27. Testing Strategy

### 27.1 Test layers

- unit tests for every policy and parser,
- contract tests for service and event schemas,
- integration tests for every Assurance Box,
- sandbox escape and isolation tests,
- end-to-end harness tests,
- model and prompt regression,
- GPU failover and capacity tests,
- security penetration tests,
- chaos tests,
- offline update tests,
- evidence verification tests,
- accessibility tests,
- Korean-language UX tests,
- compliance-control conformance tests.

### 27.2 Critical acceptance tests

1. A model requesting `.env` cannot receive the file or value.
2. A user without repository access cannot cause the model to retrieve it.
3. A README prompt injection cannot alter tool permissions.
4. A sandbox with generated malware cannot reach the host or network.
5. A protected patch cannot export while the audit sink is unavailable.
6. A prohibited GPL-family package is blocked when that policy is enabled.
7. New MD5 use in security code is blocked.
8. `System.out.println`, `print()`, and `console.log` policies apply only to configured production scope.
9. Korean resident identifiers are masked before model input and output.
10. A suspended model endpoint cannot receive new requests.
11. A code line displays complete, verifiable provenance.
12. Provenance survives file rename and ordinary refactor.
13. A modified offline update bundle is rejected.
14. An administrator cannot erase an audit event.
15. A GPU operator cannot view prompt content.
16. A prompt reviewer cannot operate GPU hosts.
17. A customer tenant cannot see another tenant’s retrieval result.
18. A sandbox is destroyed and its workspace key discarded after completion.
19. A policy exception expires automatically.
20. A compliance report differentiates automated evidence from manual assertions.

### 27.3 Security verification cadence

- continuous SAST, SCA, secret scanning, and container scanning,
- weekly dependency and image review,
- monthly threat-model delta review,
- quarterly internal red team,
- annual independent penetration test,
- model evaluation on every new artifact,
- policy regression on every bundle,
- disaster recovery exercise on defined schedule,
- incident-driven focused reassessment.

---

## 28. Operational Metrics and KPIs

### 28.1 Security

- blocked secret exposures,
- PII detections and confirmed incidents,
- prompt-injection detection and successful-containment rate,
- unauthorized context attempts,
- sandbox incidents,
- time to contain,
- policy bypass attempts,
- evidence gaps,
- model approval violations,
- dependency and license violations.

### 28.2 Governance

- percent of sessions covered by current policy packs,
- model approvals within expiry,
- control evidence freshness,
- exception count and age,
- high-risk actions with correct approvals,
- provenance coverage,
- audit verification success.

### 28.3 Developer experience

- task completion rate,
- accepted patch rate,
- accepted-with-human-modification rate,
- test pass rate,
- time to first useful action,
- approval waiting time,
- context-denial explanation satisfaction,
- Korean answer quality,
- eGovFrame benchmark success.

### 28.4 Infrastructure

- concurrent sessions,
- queue time,
- TTFT,
- tokens per second,
- GPU utilization,
- KV cache utilization,
- sandbox launch latency,
- sandbox failure rate,
- model error and fallback rate,
- evidence backlog.

Do not use raw tokens, prompts, or acceptance rate as simplistic employee monitoring.

---

## 29. Release and Offline Update System

### 29.1 Bundle contents

A GongCode offline update bundle may include:

- platform binaries,
- container and sandbox images,
- model artifacts,
- tokenizer and templates,
- policy packs,
- compliance-source metadata,
- documentation corpus,
- package mirror deltas,
- evaluation suites,
- migration scripts,
- SBOMs,
- signatures,
- rollback bundle,
- release notes.

### 29.2 Import workflow

1. Build in controlled release environment.
2. Generate SBOM and provenance.
3. Run security and license scans.
4. Sign artifacts.
5. Two-person release approval.
6. Write to approved transfer media.
7. Scan at transfer boundary.
8. Import into quarantine registry.
9. Verify signature and hash offline.
10. Show impact and migration report.
11. Run staging deployment and tests.
12. Approve production distribution.
13. Monitor canary.
14. Roll out by ring.
15. Preserve import evidence and rollback path.

### 29.3 Rings

- lab,
- internal evaluation,
- pilot agency,
- non-critical production,
- broad production,
- high-classification dedicated environments.

---

## 30. Public Website Strategy

### 30.1 Website purpose

The public website must establish GongCode as a serious public-sector engineering and security platform. It should help six audiences:

- public-sector technical leaders,
- security and compliance officers,
- developers and architects,
- procurement and program managers,
- system integrators,
- model and infrastructure teams.

The site should not be a 128-page set of thin SEO pages. Every page needs a distinct user question, technical depth, diagrams, evidence, and clear cross-navigation.

### 30.2 Website architecture

- Separate internet environment, account, CI/CD, DNS, secrets, and staff permissions.
- No network route to product deployments.
- Product telemetry never flows through the public website.
- Forms use a segregated CRM path.
- Static or heavily cached pages where practical.
- Korean primary language with complete English option.
- WCAG-aligned accessibility and Korean public web accessibility review.
- Search, sitemap, structured data, canonical URLs, and versioned docs.
- No government seal, flag treatment, or visual implication of official endorsement.
- Security claims reviewed by legal and security before publication.

### 30.3 Global navigation

```text
제품
  Harness | Control | Guard | Runtime | Gateway | Trace

보안
  Zero Trust | Sandbox | DLP | Supply Chain | Incident Response

거버넌스
  Policy Engine | Prompt Governance | Model Governance | Compliance

인프라
  Closed Network | On-Prem | GPU | CSAP Cloud | Hybrid

공공개발
  eGovFrame | KISA Secure Coding | Government Engineering Model

솔루션
  Central Government | Local Government | Public Enterprise | SI

자료
  Docs | Resources | Trust Center | Security Advisories

회사
  About | Contact
```

### 30.4 Design language

- Dark navy, warm off-white, restrained signal colors.
- Korean public-infrastructure seriousness without bureaucratic clip art.
- Diagrams and real interface mockups over generic stock photography.
- Typography optimized for Korean readability.
- Motion is purposeful: data flow, policy decision, sandbox lifecycle, provenance trace.
- Respect `prefers-reduced-motion`.
- Screens must never show real personal information, credentials, or customer data.
- AI-generated images are used for abstract infrastructure and concepts, not fake customer evidence.

### 30.5 Page template

Every public page contains:

1. title and one-sentence value proposition,
2. precise user problem,
3. product mechanism,
4. architecture or process diagram,
5. feature detail,
6. security/governance implications,
7. deployment or integration detail,
8. related pages,
9. primary CTA,
10. source/version note where discussing official frameworks or guidance.

### 30.6 Media strategy

- 15–30 second silent product loops for narrow features.
- 60–120 second narrated explainers for architecture.
- 3–5 minute technical demos for Harness, Control, Trace, Runtime, and GPU operations.
- Downloadable diagrams in accessible SVG and PDF.
- Generated imagery accompanied by explicit prompts in the page inventory below.
- Captions and transcripts for all videos.

## 31. Public Website Page Inventory (128 Pages)

Each page below has a distinct purpose. “AI image” is a production brief for generated visual assets; it is not a claim that the pictured system or customer exists. Motion must have a reduced-motion fallback, and every video must have Korean captions, an English caption track, and a transcript.

**Validated page count: 128.**


### A. Brand & Overview (8 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 1 | `/`<br>**GongCode: Secure AI Engineering for the Public Sector** | Homepage explaining the complete sovereign coding, security, governance, provenance, and GPU platform. | Full-width product hero; live Control/Harness split-screen; five proof pillars; architecture strip; deployment editions; compliance-source note; final demo CTA. | Cinematic but realistic Korean government data-center control room blended with terminal code and policy decision overlays; no flags, seals, or people in uniform. | 12-second hero loop: prompt → policy check → sandbox → code diff → provenance badge. Optional 90-second platform film. | Global nav; CTAs: Request technical briefing, View architecture. |
| 2 | `/why-gongcode`<br>**Why GongCode** | Explains why ordinary cloud coding agents do not satisfy closed-network, sovereign GPU, audit, and government governance requirements. | Problem comparison; threat model; conventional agent versus GongCode boundary diagram; procurement outcomes. | Split scene showing uncontrolled laptop agent versus governed closed-network platform with clear security boundaries. | Scroll-driven comparison that locks and highlights each missing control; 60-second explainer. | Parent: Home; links to Platform, Security, Closed Network; CTA: Compare architectures. |
| 3 | `/platform`<br>**The GongCode Platform** | Overview of Harness, Control, Guard, Runtime, Gateway, Registry, Trace, Evidence, and Evaluate. | Modular platform map; clickable product cards; shared data-flow diagram; edition matrix. | Isometric sovereign AI platform with ten labeled modules connected through signed event lines. | Module map expands on hover; 2-minute narrated platform tour. | Parent: Home; children: all product pages; CTA: Explore Harness. |
| 4 | `/public-sector-ai-development`<br>**AI Development Built for Public Institutions** | Frames GongCode around accountability, controlled execution, Korean engineering standards, and procurement realities. | Audience pain points; operating model; public-sector workflow; role-based outcomes. | Korean public-service software lifecycle illustrated as controlled stages from requirement to release. | Animated lifecycle with approval gates; interview-style video with product architect. | Parent: Why GongCode; links to Solutions and Compliance; CTA: See government workflows. |
| 5 | `/sovereign-ai`<br>**Sovereign AI Software Engineering** | Defines sovereign control over models, data, prompts, GPUs, policies, execution, and evidence. | Seven sovereignty dimensions; ownership matrix; deployment choices; model and data flow. | Layered sovereign stack located inside a secure private data center, with model, GPU, policy, and evidence layers. | Layer-by-layer reveal; 90-second sovereignty explainer. | Parent: Platform; links to On-Prem, GPU, Model Governance; CTA: Design a sovereign deployment. |
| 6 | `/closed-network-ai`<br>**AI Coding Inside Closed Networks** | Shows how coding assistance works without public internet access or cloud model dependency. | Closed-network topology; offline update workflow; internal mirrors; operational checklist. | Air-gapped network rooms connected only by approved signed media transfer. | Animated signed update bundle crossing a controlled boundary; technical demo. | Parent: Infrastructure; links to Air-Gapped, Update, Package Gate; CTA: View reference architecture. |
| 7 | `/responsible-ai`<br>**Responsible AI for Software Engineering** | Explains human authority, model evaluation, risk controls, monitoring, and grounded explanations. | Responsible AI lifecycle; risk tiers; human approvals; incident loop; measurement principles. | Human reviewer at center of a transparent AI engineering decision loop, represented abstractly and professionally. | Risk lifecycle animation; 2-minute responsible-AI overview. | Parent: Governance; links to AI Basic Act, Evaluate, Explainability; CTA: Review governance controls. |
| 8 | `/trust`<br>**Trust by Architecture, Not by Prompt** | States the core principle that models never grant themselves access and every action is externally controlled. | Model-compromise scenario; deterministic control layers; fail-closed behavior; measurable trust claims. | A glowing AI core enclosed by independent concentric authorization, runtime, and evidence boundaries. | Attack simulation animation where unsafe requests stop at each layer. | Parent: Security; links to Assurance Boxes, Zero Trust, Evidence; CTA: Inspect the control model. |

### B. Product Suite (12 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 9 | `/harness`<br>**GongCode Harness** | Terminal, IDE, and controlled web coding agent for private government environments. | Interactive terminal hero; task modes; context preview; approval flow; diff and provenance; integration cards. | High-fidelity terminal and IDE interface showing Korean prompt, context plan, and safe code change. | 30-second real product workflow loop; 4-minute developer demo. | Parent: Products; children: Harness features; CTA: Watch Harness demo. |
| 10 | `/control`<br>**GongCode Control** | Unified real-time administration, security, governance, model, GPU, provenance, and audit console. | Command-center hero; workspace navigation; incident drill-down; GPU panel; policy and provenance panels. | Realistic enterprise operations dashboard with live harnesses, security alerts, policy decisions, and GPU health. | Live data pulse animation; 5-minute administrator walkthrough. | Parent: Products; links to Security Operations, GPU, Policy Studio; CTA: Tour Control. |
| 11 | `/guard`<br>**GongCode Guard** | Deterministic enforcement for prompts, context, secrets, PII, tools, packages, networks, and exports. | Assurance Box grid; request-decision flow; rule examples; incident outcomes. | Twenty-two modular security boxes surrounding a controlled AI agent execution path. | Each box activates as a request passes; 2-minute security architecture video. | Parent: Products; children: Security Boxes; CTA: Explore all controls. |
| 12 | `/runtime`<br>**GongCode Runtime** | Disposable microVM execution for generated code, tests, builds, and tools. | MicroVM lifecycle; filesystem; network deny default; nested sandbox; destruction evidence. | Exploded technical diagram of hardened host, microVM, rootless container, and nested process sandbox. | Boot-to-destroy lifecycle animation; sandbox escape containment demo. | Parent: Products; links to Sandbox pages; CTA: View isolation design. |
| 13 | `/gateway`<br>**GongCode Gateway** | Governed inference routing for approved models and private GPU endpoints. | Request pipeline; model authorization; quotas; routing; telemetry; failover. | Private inference gateway routing sanitized requests to multiple approved GPU model pools. | Animated routing based on classification and capacity; performance dashboard clip. | Parent: Products; links to Model Routing, GPU, Model Approval; CTA: See gateway flow. |
| 14 | `/registry`<br>**GongCode Registry** | System of record for approved models, tools, policies, packages, images, and evaluations. | Artifact cards; approval status; hashes; expiry; relationships; offline import. | Secure digital registry shelves containing signed model, policy, package, and runtime artifacts. | Artifact approval timeline animation; 90-second registry tour. | Parent: Products; links to Model Approval and Update; CTA: View registry lifecycle. |
| 15 | `/trace`<br>**GongCode Trace** | Line-level AI provenance and grounded explainability for source code. | Clickable code sample; provenance drawer; context graph; replay; evidence export. | Code editor with selected lines connected to model, user, commit, context, tests, and approvals. | Click-line provenance interaction; 3-minute trace demo. | Parent: Products; children: Provenance pages; CTA: Explore a code trace. |
| 16 | `/evidence`<br>**GongCode Evidence** | Tamper-evident audit records and continuous certification evidence. | Evidence bundle anatomy; retention; verification; auditor export; control freshness. | Signed evidence packages flowing into an immutable vault with verification seals that are generic, not governmental. | Bundle assembly and signature verification animation. | Parent: Products; links to Audit Readiness and Evidence Export; CTA: See evidence model. |
| 17 | `/connect`<br>**GongCode Connect** | Controlled integrations with Git, CI/CD, identity, package, document, and security systems. | Integration topology; connector catalog; permissions; event contracts; offline patterns. | Secure connectors linking GongCode to GitLab, Jenkins, Nexus, identity, SIEM, and document systems. | Connection lines illuminate only after policy authorization. | Parent: Products; links to integration detail in Docs; CTA: Review integrations. |
| 18 | `/evaluate`<br>**GongCode Evaluate** | Evaluation platform for models, agents, security behavior, Korean coding, and regression. | Evaluation packs; scorecards; approval gates; red-team cases; comparison view. | Benchmark control room comparing approved coding models across security, Korean, and eGovFrame tasks. | Benchmark runs animate into approval decision; 2-minute evaluation overview. | Parent: Products; links to Government Evaluation and Model Governance; CTA: View evaluation framework. |
| 19 | `/update`<br>**GongCode Update** | Signed offline updates for software, models, policy packs, documents, and mirrors. | Bundle contents; dual approval; transfer boundary; quarantine; canary; rollback. | Tamper-evident update package moving through scan, signature, quarantine, and controlled deployment stages. | Offline update lifecycle animation; operator demo. | Parent: Products; links to Air-Gapped and Supply Chain; CTA: See update process. |
| 20 | `/developer-experience`<br>**A Government-Grade Developer Experience** | Explains how security remains understandable and fast for developers. | Task plan; context preview; exact approval moment; Korean explanations; fast low-risk path. | Developer terminal with clear green, amber, and red policy explanations instead of opaque errors. | Interactive approval-flow prototype; developer testimonial-style demo without fabricated customer claims. | Parent: Products; links to Harness and Policy Builder; CTA: Follow a developer task. |

### C. Harness Capabilities (20 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 21 | `/terminal-agent`<br>**Secure Terminal Coding Agent** | CLI-native AI coding with identity-bound sessions and controlled tools. | Terminal sequence from login to plan, execution, test, and signed patch; command risk labels. | Minimal dark terminal with Korean task request and structured authorization cards. | Keystroke-paced workflow loop; 3-minute CLI demo. | Parent: Harness; links to Secure Shell and Human Approval; CTA: View CLI workflow. |
| 22 | `/ide-extension`<br>**GongCode in the IDE** | VS Code and eGovFrame/Eclipse integration without bypassing central controls. | IDE sidebar; inline provenance; context selector; policy findings; diff review. | High-fidelity code editor with GongCode panel, Korean explanations, and provenance badges. | Inline badge and context animation; 2-minute IDE demo. | Parent: Harness; links to Provenance and eGovFrame Assistant; CTA: Explore IDE integration. |
| 23 | `/task-planning`<br>**Policy-Aware Task Planning** | The agent exposes intended files, commands, packages, network, and approvals before action. | Task card; proposed context; action sequence; risk score; editable scope. | Structured AI plan displayed like a government change request with technical details. | Plan expands into authorized steps; 60-second feature loop. | Parent: Harness; links to Human Approval and Policy Engine; CTA: See a task plan. |
| 24 | `/repo-understanding`<br>**Repository Understanding** | Builds a permission-filtered map of symbols, dependencies, APIs, data flows, and security boundaries. | Repository graph; module summary; architecture view; access-filter demonstration. | Large Java public-system repository visualized as modules, APIs, database, and authorization paths. | Graph assembles from authorized files only; technical demo. | Parent: Harness; links to Context Selection and Context Map; CTA: Explore repository intelligence. |
| 25 | `/context-selection`<br>**Visible, Governed Context Selection** | Users see why each file or document is requested before it reaches the model. | Context candidate list; sensitivity labels; transformations; token budget; denial reasons. | Context cards passing through secrets, PII, and permission filters before model input. | Drag-to-remove context; blocked card visibly stops; 90-second demo. | Parent: Harness; links to Context Box and Secrets Box; CTA: Inspect context controls. |
| 26 | `/code-generation`<br>**Governed Code Generation** | Generates code in an isolated workspace with policy, tests, and provenance attached. | Request-to-patch flow; coding standards; scan results; review and export. | Code emerging inside a transparent isolated microVM, surrounded by policy checks. | Generated diff gains test and policy badges; 2-minute demo. | Parent: Harness; links to Runtime and Code Lineage; CTA: Follow generated code. |
| 27 | `/code-refactoring`<br>**Multi-File Refactoring** | Coordinates safe refactors across modules while preserving behavior and provenance. | Dependency map; proposed files; incremental patches; tests; rollback. | Interconnected Java modules being refactored with preserved contracts and test coverage. | Before/after dependency animation; 2-minute refactor demo. | Parent: Harness; links to Repository Understanding and Test Generation; CTA: View refactor workflow. |
| 28 | `/test-generation`<br>**Test Generation and Verification** | Creates targeted unit and integration tests and records exactly how code was verified. | Coverage gaps; generated tests; sandbox run; failure loop; evidence output. | Test matrix connected to code paths, security rules, and passing evidence. | Tests execute with live results; 90-second demo. | Parent: Harness; links to Secure Coding and Evidence; CTA: See verification. |
| 29 | `/debugging`<br>**Controlled Debugging** | Analyzes logs and failures while filtering secrets, PII, and production access. | Sanitized logs; hypothesis list; reproducible test; fix; validation. | Debug timeline with sensitive values automatically tokenized and linked to code. | Log-to-root-cause animation; 2-minute debugging demo. | Parent: Harness; links to PII Box, Secrets Box, Logging Agent; CTA: Debug safely. |
| 30 | `/migration-assistant`<br>**Legacy Modernization Assistant** | Plans and executes controlled migrations for public-sector Java, JSP, XML, databases, and deployment systems. | Current-state inventory; compatibility risks; migration waves; test evidence. | Legacy monolith transforming into modern services while preserving audited interfaces. | Phased migration diagram animates by wave; 3-minute example. | Parent: Harness; links to eGovFrame and Government Java; CTA: Assess a legacy system. |
| 31 | `/dependency-management`<br>**Approved Dependency Management** | Requests packages through internal mirrors, license policy, signatures, and vulnerability checks. | Dependency request; mirror lookup; license; vulnerability; approval; SBOM. | Package moving through a secure inspection conveyor with license and vulnerability gates. | Package approval animation; 90-second supply-chain demo. | Parent: Harness; links to Package Box and License Box; CTA: See package governance. |
| 32 | `/secure-shell`<br>**A Shell Without Unrestricted Authority** | Provides build and test compatibility while commands remain parsed, scoped, and policy-controlled. | Command preview; parser; policy decision; constrained execution; output scan. | Terminal command entering a command firewall before a disposable sandbox. | Command tokens separate and receive allow/block labels. | Parent: Harness; links to Command Box and Runtime; CTA: Inspect shell controls. |
| 33 | `/human-approval`<br>**Human Approval at the Point of Risk** | Requires clear, unbundled approval for material code, dependency, network, and environment changes. | Risk-specific approval card; reviewer evidence; decision; expiry. | Two reviewers examining an authentication diff with policy and test evidence. | Approval card reveals exact changed capabilities; 60-second demo. | Parent: Harness; links to Approval Workflows; CTA: Review approval design. |
| 34 | `/multi-agent-review`<br>**Multi-Agent Review Under One Policy** | Uses specialized coding, security, architecture, and testing agents without multiplying authority. | Agent roles; shared evidence; independent findings; human resolution. | Four specialized AI review agents around one controlled patch and policy boundary. | Agents review in parallel; disagreements converge into a human decision. | Parent: Harness; links to Evaluate and Model Governance; CTA: See review orchestration. |
| 35 | `/offline-docs`<br>**Offline Documentation Intelligence** | Retrieves approved framework, API, policy, and architecture documentation without public web access. | Document registry; version labels; permission filters; citations; update process. | Secure internal technical library with versioned documents feeding cited answers. | Citation paths animate from document to answer; 90-second demo. | Parent: Harness; links to Air-Gapped and Government Model; CTA: Explore offline knowledge. |
| 36 | `/korean-code-explanation`<br>**Korean-First Code Explanation** | Explains code, architecture, risk, and policy naturally in Korean while preserving technical precision. | Code selection; Korean explanation; glossary; cited policy; English toggle. | Code editor beside concise Korean architectural explanation and terminology cards. | Selected code highlights corresponding Korean explanation. | Parent: Harness; links to Government AI Model; CTA: See Korean explanations. |
| 37 | `/egovframe-assistant`<br>**eGovFrame Development Assistant** | Understands eGovFrame project structures, components, lifecycle tools, and modernization paths. | Project wizard awareness; runtime layers; common components; examples; version handling. | eGovFrame application layers with GongCode assistant mapping controllers, services, persistence, integration, and batch. | Layer mapping animation; 3-minute eGovFrame task demo. | Parent: Harness; links to eGovFrame intelligence page; CTA: View eGovFrame capabilities. |
| 38 | `/spring-assistant`<br>**Spring and Spring Security Assistant** | Supports secure Spring applications, validation, transactions, tests, and authorization. | Architecture patterns; secure endpoint example; dependency and test checks. | Spring service architecture with authentication and policy control highlighted. | Request flow animates through security filters and service layers. | Parent: Harness; links to Secure Coding and Government Java; CTA: Explore Spring support. |
| 39 | `/mybatis-assistant`<br>**MyBatis and SQL Assistant** | Analyzes mapper XML, dynamic SQL, transaction boundaries, performance, and injection risks. | Mapper-to-service map; query review; parameter safety; execution plan; tests. | MyBatis mapper XML connected to service code and database with unsafe dynamic SQL highlighted. | Unsafe query transforms into parameterized form; 90-second demo. | Parent: Harness; links to Government Database; CTA: Review database code. |
| 40 | `/batch-systems`<br>**Government Batch Systems** | Supports scheduled jobs, large data processing, restartability, audit, and operational safety. | Batch topology; job plan; checkpoint; failure recovery; data-access logging. | Nightly public-system batch workflow with checkpoints, retries, audit events, and isolated execution. | Job timeline animates through restart and evidence capture. | Parent: Harness; links to Government Java and PII Control; CTA: See batch workflow. |

### D. Security & Assurance Boxes (22 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 41 | `/security`<br>**GongCode Security Architecture** | Complete overview of zero trust, Assurance Boxes, sandboxing, model isolation, evidence, and incident response. | Threat model hero; layered architecture; Assurance Box directory; measurable control claims. | Technical cutaway of developer, control, sandbox, GPU, and evidence trust zones. | Attack path simulation stopped across layers; 3-minute architecture video. | Parent: Security; children: all security pages; CTA: Download security architecture. |
| 42 | `/security/zero-trust`<br>**Zero Trust for AI Coding Agents** | Applies continuous identity, device, service, workload, action, and resource authorization. | Trust decisions by layer; identity flow; ABAC examples; degraded mode. | Every service and workload linked through short-lived certificates and explicit authorization. | Connections appear only after identity and policy validation. | Parent: Security; links to Identity Box and Network Box; CTA: Review trust model. |
| 43 | `/security/identity-box`<br>**Identity Box** | Binds every prompt, tool request, approval, and artifact to a verified actor and role. | SSO flow; RBAC/ABAC; delegated access; break-glass; audit record. | Identity graph linking user, device, organization, project, session, and action. | Identity attributes resolve into task permissions. | Parent: Security; links to Device Box and Identity & Access docs; CTA: See identity controls. |
| 44 | `/security/prompt-box`<br>**Prompt Box** | Classifies, redacts, templates, retains, and authorizes prompts before model access. | Prompt envelope; classification; sensitive transformations; retention choices. | Korean prompt moving through classification, redaction, and approved template layers. | Prompt fragments receive data labels; 90-second prompt-governance demo. | Parent: Security; links to Prompt Governance; CTA: Inspect prompt handling. |
| 45 | `/security/context-box`<br>**Context Box: The Context Firewall** | Limits model context to necessary, authorized, transformed data. | Candidate context; file and span decisions; token budget; denial explanation. | Repository files passing through a firewall that permits only approved code spans. | Files split into spans and filtered; interactive context example. | Parent: Security; links to Context Selection and PII Box; CTA: See the context firewall. |
| 46 | `/security/secrets-box`<br>**Secrets Box** | Prevents credentials, certificates, keys, tokens, and connection strings from reaching models or exports. | Detection types; tokenization; secret broker; command redaction; incident flow. | Secret values transforming into safe opaque tokens before prompt and command use. | Secret detected, replaced, and revoked animation; security demo. | Parent: Security; links to Secrets Broker and DLP; CTA: Review secret protections. |
| 47 | `/security/pii-box`<br>**PII Box for Korean Personal Information** | Detects and controls Korean identifiers and sensitive personal data in code, logs, prompts, and outputs. | Data categories; detection; purpose limitation; masking; retention and incident handling. | Abstract Korean data records with resident, passport, phone, financial, and biometric fields safely masked. | Fields progressively minimize before model context; 2-minute demo. | Parent: Security; links to PIPA Compliance and Data Governance; CTA: Explore PII controls. |
| 48 | `/security/injection-box`<br>**Prompt Injection Defense** | Treats instructions embedded in repositories, documents, logs, and web content as untrusted data. | Attack examples; trust labels; tool-capability separation; red-team results. | README and issue text containing malicious instructions stopped before the agent tool layer. | Injection attempt visually loses authority and is quarantined. | Parent: Security; links to Red Team and Context Box; CTA: Test injection defenses. |
| 49 | `/security/file-box`<br>**File Box** | Enforces repository, branch, path, classification, and read/write permissions. | Path rules; code owners; metadata-only mode; protected files; audit events. | Repository tree with allowed, read-only, metadata-only, and denied zones. | Path permissions color and explain on selection. | Parent: Security; links to Context Box and Branch Governance docs; CTA: See file permissions. |
| 50 | `/security/command-box`<br>**Command Box** | Parses and controls shell, compiler, test, migration, and tool commands. | Command AST; allow/deny rules; environment; risk; constrained execution. | Shell command decomposed into executable, arguments, paths, and network implications. | Command tokens pass or stop at individual policy gates. | Parent: Security; links to Secure Shell and Runtime; CTA: Inspect command authorization. |
| 51 | `/security/network-box`<br>**Network Box** | Blocks network access by default and brokers explicit approved destinations. | No-route baseline; egress proxy; destination identity; content and byte limits. | Sandbox with no network, then a single narrow authenticated path to an internal mirror. | Network path appears only for one approved purpose and expires. | Parent: Security; links to Network Isolation and Closed Network; CTA: View egress policy. |
| 52 | `/security/package-box`<br>**Package Box** | Allows only approved, scanned, pinned dependencies from internal mirrors. | Request; mirror; checksum; malware; vulnerability; approval; SBOM. | Dependency package inspected by multiple automated stations before entering the sandbox. | Inspection results accumulate into an allow or deny decision. | Parent: Security; links to Dependency Management and Supply Chain; CTA: See package flow. |
| 53 | `/security/license-box`<br>**License Box** | Enforces organization-specific open-source and commercial license policy. | SPDX policy; transitive dependencies; obligations; exceptions; evidence. | Dependency graph with license labels and a prohibited transitive license highlighted. | Graph traces the license obligation to its parent package. | Parent: Security; links to Policy Builder; CTA: Configure license rules. |
| 54 | `/security/model-box`<br>**Model Box** | Prevents use of unapproved models, artifacts, endpoints, classifications, or inference settings. | Approval matrix; model hash; classification; expiry; request decision. | Model cards entering an authorization gate tied to data classification and task type. | Approved model lights up; suspended model route closes. | Parent: Security; links to Model Approval and Registry; CTA: See model authorization. |
| 55 | `/security/runtime-box`<br>**Runtime Box** | Creates and monitors the disposable execution boundary for agent actions. | Launch specification; attestation; resources; runtime detection; destruction. | MicroVM instance with verified image, limited resources, no network, and external monitoring. | Sandbox lifecycle from signed spec to cryptographic destruction. | Parent: Security; links to Runtime and MicroVM; CTA: Inspect runtime controls. |
| 56 | `/security/provenance-box`<br>**Provenance Box** | Records how every AI-assisted code span was created, verified, reviewed, and changed. | Event linkage; span mapping; signatures; Git references; query example. | Code spans connected to session, model, context, policies, tests, and reviewers. | Trace graph builds as the patch evolves. | Parent: Security; links to Trace and Code Lineage; CTA: Follow code provenance. |
| 57 | `/security/evidence-box`<br>**Evidence Box** | Continuously builds signed evidence for sessions, releases, incidents, and compliance controls. | Evidence inputs; bundle; signature; retention; auditor verification. | Multiple signed logs and scan reports assembled into one verifiable evidence package. | Evidence bundle locks and receives a verifiable hash. | Parent: Security; links to Audit Readiness and Evidence Export; CTA: Verify an evidence bundle. |
| 58 | `/security/logging-agent`<br>**Tamper-Evident Logging Agent** | Streams security and operational events outside sandboxes before local compromise can alter them. | Event path; local signed buffer; external sink; outage behavior; access controls. | Sandbox event stream flowing one way into a separate immutable logging zone. | Events chain cryptographically; outage triggers fail-closed indicator. | Parent: Security; links to Evidence and Incident Response; CTA: Review audit logging. |
| 59 | `/security/dlp`<br>**Data Loss Prevention** | Inspects prompts, responses, commands, network traffic, files, and exported artifacts for sensitive data. | Inspection points; content classes; block/redact/quarantine; investigation workflow. | Sensitive information attempting multiple exit paths, each stopped by a coordinated DLP layer. | Data paths animate and block at prompt, network, and export gates. | Parent: Security; links to PII and Secrets; CTA: Map DLP controls. |
| 60 | `/security/supply-chain`<br>**AI and Software Supply-Chain Security** | Protects source, packages, models, containers, policies, and offline updates. | Artifact chain of custody; signatures; SBOM; reproducible build; revocation. | End-to-end chain from source commit to signed deployment artifact and model weights. | Each artifact hash links into one provenance chain. | Parent: Security; links to Package, Registry, Update; CTA: View supply-chain controls. |
| 61 | `/security/incident-response`<br>**Incident Response for AI Engineering** | Contains sessions, preserves evidence, revokes credentials, and coordinates recovery. | Alert; triage; blast radius; containment; forensics; policy change; closure. | Security operations timeline centered on an isolated coding session and related assets. | One-click containment sequence; 3-minute incident simulation. | Parent: Security; links to Control and Evidence; CTA: Run the incident walkthrough. |
| 62 | `/security/red-team`<br>**Red Teaming GongCode** | Explains adversarial testing for models, prompts, tools, sandboxes, packages, and administrators. | Attack library; evaluation lab; findings; regression; disclosure policy. | Controlled adversarial lab testing prompt injection, exfiltration, sandbox escape, and policy evasion. | Attack cards run against a test environment and produce regression cases. | Parent: Security; links to Evaluate and Security Advisories; CTA: Review red-team program. |

### E. Governance & Compliance (16 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 63 | `/governance`<br>**AI Engineering Governance** | Unifies model, prompt, data, policy, approval, provenance, incident, and retirement governance. | Governance lifecycle; accountable roles; decision records; evidence and review cadence. | Transparent governance wheel linking use case, model, data, policy, people, and evidence. | Lifecycle rotates through approval, monitoring, incident, and retirement. | Parent: Governance; children: governance pages; CTA: Review governance operating model. |
| 64 | `/policy-engine`<br>**Deterministic Policy Engine** | Evaluates every protected action outside the model with signed, explainable decisions. | Action envelope; rule evaluation; obligations; decision; evidence. | AI tool request entering a deterministic policy engine and emerging allowed, transformed, or denied. | Rule evaluation stack animates with reason codes. | Parent: Governance; links to Policy-as-Code and Policy Builder; CTA: See decision flow. |
| 65 | `/policy-as-code`<br>**Policy as Code** | Versions, tests, simulates, signs, and deploys organizational security and engineering rules. | Repository workflow; schema; tests; historical simulation; rollout rings. | Policy source code moving through tests, review, signature, and staged enforcement. | Diff-to-impact animation; technical tutorial video. | Parent: Governance; links to Policy Engine and Custom Packs; CTA: View policy lifecycle. |
| 66 | `/policy-builder`<br>**Visual Policy Builder** | Lets authorized administrators configure checkboxes, thresholds, approvals, and scope without writing code. | Policy categories; checkbox examples; inherited rule source; conflict warning; preview. | High-fidelity policy settings screen for licenses, crypto, logging, tests, approvals, and network. | Selecting a checkbox updates a live impact preview. | Parent: Governance; links to Policy-as-Code and Compliance; CTA: Explore policy settings. |
| 67 | `/approval-workflows`<br>**Risk-Based Approval Workflows** | Routes sensitive code, model, package, network, policy, and release actions to accountable reviewers. | Approval matrix; evidence card; separation of duties; expiry; delegation. | Approval graph showing developer, technical reviewer, security officer, and model approver. | Approval route changes based on code risk and classification. | Parent: Governance; links to Human Approval and Identity; CTA: Design an approval matrix. |
| 68 | `/prompt-governance`<br>**Prompt Governance** | Controls templates, classification, redaction, access, retention, sampling, and review. | Prompt registry; version compare; data policy; retention modes; review access. | Prompt history represented as controlled versions with sensitive sections redacted. | Version comparison and retention-expiry animation. | Parent: Governance; links to Prompt Box and Data Governance; CTA: Review prompt controls. |
| 69 | `/model-governance`<br>**Model Governance** | Manages model intake, evaluation, approval, deployment, monitoring, suspension, and retirement. | Model lifecycle; approval status; risk tiers; incidents; reapproval. | Model artifacts moving through evaluation labs to signed deployment pools. | Lifecycle animation with canary, monitor, suspend, retire states. | Parent: Governance; links to Model Approval and Evaluate; CTA: Inspect model lifecycle. |
| 70 | `/data-governance`<br>**Data Governance for Coding AI** | Controls repository, prompt, document, log, personal-data, retrieval, and training-data use. | Data categories; purpose; flow; retention; deletion; training prohibition. | Data lineage map from source repository and documents through context, model, logs, and evidence. | Data paths display purpose, transformation, and retention. | Parent: Governance; links to PII Box and Context Firewall; CTA: Map data flows. |
| 71 | `/retention`<br>**Retention, Deletion, and Legal Hold** | Defines different lifecycles for raw prompts, redacted content, logs, provenance, and evidence. | Retention matrix; expiry; deletion propagation; legal hold; verification. | Timeline of prompt, log, provenance, and evidence objects expiring at different intervals. | Objects expire and indexes update; legal hold freezes selected records. | Parent: Governance; links to Evidence and PIPA; CTA: Configure retention. |
| 72 | `/audit-readiness`<br>**Continuous Audit Readiness** | Builds evidence during normal engineering work instead of assembling screenshots later. | Control-to-evidence matrix; freshness; gaps; auditor package; verification. | Live compliance matrix connected to signed operational evidence. | Control cells change from stale to current as evidence arrives. | Parent: Governance; links to Evidence Box and Compliance Center; CTA: View evidence mapping. |
| 73 | `/compliance/kisa`<br>**KISA and MOIS Secure Development Profile** | Maps GongCode controls to Korean software development security guidance and weakness diagnostics. | Source/version banner; weakness categories; preventive rules; scans; evidence; limitations. | Secure Korean public software lifecycle with design, implementation, test, and remediation stages. | Vulnerability examples move from detection to policy-backed remediation. | Parent: Compliance; links to Secure Coding KISA; CTA: Review control map. |
| 74 | `/compliance/csap`<br>**CSAP Readiness** | Explains service scope, asset inventory, control evidence, tenant isolation, operations, and certification boundaries. | Certification disclaimer; service editions; control map; evidence; assessment workflow. | Public-sector cloud service boundary with clearly scoped systems, assets, people, and support processes. | Scope boundary animates and highlights included assets. | Parent: Compliance; links to CSAP Cloud and Trust Center; CTA: Request CSAP architecture briefing. |
| 75 | `/compliance/isms-p`<br>**ISMS-P Control Mapping** | Maps management, protection, and personal-information control domains to GongCode evidence. | Domain matrix; automated versus organizational controls; owners; evidence freshness. | Three-layer ISMS-P control map connected to platform and process evidence. | Matrix drill-down animation; no claim of certification. | Parent: Compliance; links to Audit Readiness and PIPA; CTA: Explore ISMS-P evidence. |
| 76 | `/compliance/pipa`<br>**Personal Information Protection** | Shows access, minimization, encryption, records, incident, and public-system controls for personal data. | Current-source banner; data flow; PII Box; access logs; retention; evidence. | Personal information lifecycle protected through collection, use, access, model context, and deletion. | Data lifecycle animation with control checkpoints. | Parent: Compliance; links to PII Box and Data Governance; CTA: Review privacy controls. |
| 77 | `/compliance/ai-basic-act`<br>**Korean AI Basic Act Readiness** | Supports AI inventory, risk management, safety monitoring, impact assessment, transparency, and evidence workflows. | Effective-date/source banner; applicability; lifecycle controls; high-impact workflow; reports. | AI system governance lifecycle with risk identification, evaluation, monitoring, and incident response. | Risk register evolves through model lifecycle; 2-minute overview. | Parent: Compliance; links to Responsible AI and Model Governance; CTA: Review AI governance workflow. |
| 78 | `/compliance/custom-packs`<br>**Custom Agency Policy Packs** | Lets agencies encode local coding, licensing, cryptography, approval, retention, and architecture standards. | Pack inheritance; settings; advanced rules; simulation; signature; rollout. | Multiple agency policy layers inheriting a mandatory national baseline without weakening it. | Layered policies merge with conflict explanation. | Parent: Compliance; links to Policy Studio and Policy-as-Code; CTA: Design a policy pack. |

### F. Provenance & Explainability (10 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 79 | `/provenance`<br>**AI Provenance for Source Code** | Overview of repository, commit, file, symbol, and line-level attribution. | Clickable code hero; provenance graph; human/AI mix; evidence and replay. | Source code connected through transparent lines to user, model, context, tools, policy, tests, and commit. | Select a line and expand its complete trace. | Parent: Trace; children: provenance pages; CTA: Explore a provenance record. |
| 80 | `/provenance/code-lineage`<br>**Code Lineage** | Tracks how AI-generated code survives renames, refactors, merges, and human edits. | Timeline of code span; AST fingerprint; mixed attribution; ambiguity review. | One code function evolving across commits while its provenance remains attached. | Function moves between files and the trace follows. | Parent: Provenance; links to Human-AI Attribution; CTA: Follow code across commits. |
| 81 | `/provenance/commit-view`<br>**AI-Aware Commit View** | Shows AI contribution, sessions, reviewers, policies, tests, and evidence for each commit. | Commit summary; file heatmap; AI percentage; findings; evidence link. | Git commit interface enriched with provenance, verification, and approval panels. | Commit heatmap animates by provenance source. | Parent: Provenance; links to Evidence Export; CTA: Inspect an AI-aware commit. |
| 82 | `/provenance/model-card`<br>**Model Information on Every Change** | Surfaces exact model artifact, approval, evaluation, inference settings, and limitations behind code. | Model identity; hash; approval; benchmark; endpoint; expiry; related spans. | Model card connected to code changes and approval scope. | Model version switch shows affected commits. | Parent: Provenance; links to Model Governance and Registry; CTA: View model record. |
| 83 | `/provenance/context-map`<br>**Context Map** | Visualizes every file, document, issue, API, and log that influenced an AI-assisted change. | Graph; source trust; sensitivity; selected spans; transformations; citations. | Permission-filtered context graph centered on a code patch. | Graph nodes reveal reason and transformation on hover. | Parent: Provenance; links to Context Firewall; CTA: Explore a context graph. |
| 84 | `/provenance/tool-trace`<br>**Tool and Command Trace** | Reconstructs file reads, writes, shell commands, tests, network requests, and package actions. | Chronological trace; policy decisions; output; resource use; replay. | Precise engineering event timeline with commands and allow/block decisions. | Timeline scrubbing replays sandbox activity. | Parent: Provenance; links to Command Box and Replay; CTA: Replay tool activity. |
| 85 | `/provenance/human-ai-attribution`<br>**Human and AI Attribution** | Distinguishes generated, human-edited, AI-refactored, reviewed, and template-derived code. | Attribution legend; mixed span; reviewer action; ownership implications. | Code diff colored by contribution source with accessible patterns, not color alone. | Slider moves from initial AI patch to final human-reviewed code. | Parent: Provenance; links to Code Lineage; CTA: Inspect attribution. |
| 86 | `/provenance/explanations`<br>**Grounded Code Explanations** | Explains why code exists, what influenced it, how it was verified, and what remains uncertain. | Question tabs; cited inputs; tests; policies; limitations; no hidden reasoning claim. | Code explanation panel grounded in documents, context, and test evidence. | Explanation sections connect to exact evidence. | Parent: Provenance; links to Responsible AI; CTA: See a grounded explanation. |
| 87 | `/provenance/evidence-export`<br>**Provenance Evidence Export** | Exports signed, permission-filtered records for auditors, customers, incidents, and releases. | Export scope; redaction; manifest; signature; verification tool. | Portable signed evidence package with encrypted references and a verification manifest. | Export assembles selected trace elements and verifies hash. | Parent: Provenance; links to Evidence; CTA: View export format. |
| 88 | `/provenance/replay`<br>**Reproducible AI Session Replay** | Recreates a session using pinned repository, model, policy, sandbox, documents, and tools. | Replay prerequisites; version manifest; comparison; divergence report. | Time-capsule view containing exact code, model, policy, toolchain, and context versions. | Replay runs beside original and highlights divergence. | Parent: Provenance; links to Tool Trace and Offline Update; CTA: See replay architecture. |

### G. Infrastructure, GPU & Sandbox (16 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 89 | `/infrastructure`<br>**GongCode Infrastructure** | Overview of closed-network control plane, execution plane, model plane, supply chain, and evidence plane. | Trust-zone topology; deployment editions; network boundaries; operational responsibilities. | Detailed private data-center architecture with separated control, sandbox, GPU, registry, and evidence zones. | Data and control flows animate independently. | Parent: Infrastructure; children: infrastructure pages; CTA: Download reference architecture. |
| 90 | `/infrastructure/air-gapped`<br>**Air-Gapped GongCode** | Runs the complete platform without public internet or required vendor telemetry. | Air-gap topology; local services; update transfer; support workflow; operational checklist. | Two disconnected environments with signed, scanned update media crossing a guarded transfer process. | Offline bundle journey; 3-minute operator walkthrough. | Parent: Infrastructure; links to Update and Closed Network; CTA: Review air-gap design. |
| 91 | `/infrastructure/on-prem`<br>**On-Premises Deployment** | Customer-owned control, models, GPUs, data, evidence, keys, and operations. | Sizing profiles; HA; responsibilities; installation; integration; lifecycle. | Modern Korean public data center racks hosting GongCode services and private GPU nodes. | Layered deployment build-up; architecture briefing video. | Parent: Infrastructure; links to GPU and HA/DR; CTA: Plan on-prem deployment. |
| 92 | `/infrastructure/csap-cloud`<br>**CSAP Cloud Architecture** | Describes the separately assessed cloud edition, service scope, tenancy, evidence, and operations. | Scope diagram; shared responsibility; controls; support; certification process disclaimer. | Public-sector cloud boundary with segregated tenants, services, evidence, and operations zones. | Scope overlay toggles by service component. | Parent: Infrastructure; links to CSAP Readiness; CTA: Request cloud architecture review. |
| 93 | `/infrastructure/hybrid`<br>**Hybrid Sovereign Deployment** | Splits control, data, evidence, and model workloads only through approved, classified interfaces. | Four hybrid patterns; data-class restrictions; gateway; latency; provenance location. | Customer data center and approved GPU facility linked by one encrypted governed channel. | Toggle between hybrid patterns and show permitted data paths. | Parent: Infrastructure; links to Gateway and Data Governance; CTA: Compare hybrid patterns. |
| 94 | `/infrastructure/gpu`<br>**Private GPU Infrastructure** | Explains dedicated, pooled, and partitioned GPU models for secure coding workloads. | GPU zones; node controls; model pools; metrics; capacity profiles. | Private H100-class GPU cluster with separate management and inference networks. | Live-like utilization and queue animation; 2-minute infrastructure tour. | Parent: Infrastructure; links to GPU Scheduler and Model Routing; CTA: Size a GPU fleet. |
| 95 | `/infrastructure/gpu-scheduler`<br>**GPU Scheduling and Admission Control** | Prioritizes concurrent sessions by agency, classification, deadline, model, and resource budget. | Queue lanes; reservations; preemption policy; quotas; capacity forecast. | Multiple agency request queues entering governed GPU scheduling lanes. | Requests reorder based on priority and approved reservations. | Parent: GPU; links to Control GPU view; CTA: Inspect scheduling logic. |
| 96 | `/infrastructure/vllm`<br>**vLLM Deployment** | Reference integration for governed high-throughput model serving. | Gateway boundary; endpoint deployment; caching; metrics; approval and rollback. | vLLM endpoint pool behind GongCode Gateway with no direct sandbox access. | Requests batch and route while policy remains outside serving engine. | Parent: GPU; links to Gateway and Model Routing; CTA: View vLLM architecture. |
| 97 | `/infrastructure/sglang`<br>**SGLang Deployment** | Reference integration for structured agent workloads and high-performance serving. | Endpoint topology; runtime options; caching; evaluation; canary. | SGLang model pool integrated into the same approved gateway and GPU control plane. | Serving-engine comparison animation without unsupported benchmark claims. | Parent: GPU; links to Gateway and Evaluate; CTA: Compare serving options. |
| 98 | `/infrastructure/model-routing`<br>**Classification-Aware Model Routing** | Selects models by task, data class, approval, quality, context, and capacity. | Routing decision tree; fallback; no-downgrade rule; telemetry. | Sanitized request routed among approved Korean coding, general coding, and review models. | Decision branches illuminate with reason codes. | Parent: Infrastructure; links to Model Box and Gateway; CTA: See routing decisions. |
| 99 | `/infrastructure/ha-dr`<br>**High Availability and Disaster Recovery** | Protects control, policy, audit, model, provenance, and evidence services from failure. | HA topology; RPO/RTO; backup; restore; site failover; test schedule. | Dual-site architecture with replicated control and evidence, plus independent GPU capacity. | Failure shifts services to recovery site and verifies evidence integrity. | Parent: Infrastructure; links to Operations docs; CTA: Review recovery objectives. |
| 100 | `/sandbox`<br>**Secure Agent Sandbox** | Overview of microVM isolation, restricted tools, ephemeral storage, no-network default, and export controls. | Layered runtime; lifecycle; filesystem; network; monitoring; destruction. | Exploded secure sandbox stack with clear host, VM, container, process, and policy boundaries. | Boot, execute, scan, export, destroy animation. | Parent: Runtime; children: sandbox pages; CTA: Explore sandbox security. |
| 101 | `/sandbox/microvm`<br>**MicroVM-per-Session Isolation** | Uses a separate guest kernel and ephemeral environment for sensitive agent execution. | MicroVM versus container comparison; launch spec; attestation; overhead considerations. | Side-by-side technical comparison of shared-kernel container and isolated microVM. | Threat path stops at guest boundary; benchmark caveat panel. | Parent: Sandbox; links to Runtime Box; CTA: Compare isolation levels. |
| 102 | `/sandbox/network-isolation`<br>**Sandbox Network Isolation** | Provides no route by default and brokered access to named internal services. | Network topology; egress authorization; DNS/certificate checks; transfer limits. | Sandbox surrounded by an empty network boundary with narrow temporary service tunnels. | Authorized path appears and expires after use. | Parent: Sandbox; links to Network Box; CTA: View network controls. |
| 103 | `/sandbox/secrets-broker`<br>**Short-Lived Secret Brokering** | Injects target-scoped credentials only into the approved process and never into model context. | Request; approval; issuance; process injection; revocation; audit. | Opaque temporary token delivered directly to one process, bypassing workspace and prompt. | Token appears for seconds, is used, then revoked. | Parent: Sandbox; links to Secrets Box; CTA: Follow a credential request. |
| 104 | `/sandbox/windows-linux`<br>**Linux and Windows Sandbox Pools** | Supports distinct hardened execution pools without weakening the common policy and evidence model. | Linux baseline; Windows VM pool; toolchain images; capability parity; limitations. | Parallel Linux microVM and Windows ephemeral VM connected to one policy and evidence plane. | Task routes to the correct pool based on toolchain and policy. | Parent: Sandbox; links to Deployment Editions; CTA: Review platform support. |

### H. Government Engineering Intelligence (10 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 105 | `/government-ai-model`<br>**A Coding Model for Korean Government Engineering** | Explains the fine-tuning, retrieval, evaluation, and policy strategy for public-sector software. | Model layers; data sources; RAG versus SFT; evaluations; no-customer-training default. | Korean coding model surrounded by eGovFrame, secure coding, API, database, and policy knowledge sources. | Knowledge sources route to training, retrieval, or policy layers. | Parent: Public Development; links to Evaluate and Model Governance; CTA: Review model strategy. |
| 106 | `/egovframe`<br>**eGovFrame 5.0 Intelligence** | Detailed support for current eGovFrame development, runtime layers, templates, common components, and modernization. | Version banner; architecture layers; supported tasks; migration; evaluation cases. | Technical eGovFrame layer diagram enhanced with GongCode assistance points. | Layer-by-layer supported-task animation; 4-minute technical demo. | Parent: Public Development; links to eGovFrame Assistant; CTA: Explore eGovFrame support. |
| 107 | `/secure-coding-kisa`<br>**Korean Secure Coding Intelligence** | Turns public software development security guidance into review, generation, tests, and evidence. | Weakness families; code examples; policy rules; scanners; remediation; evidence. | Vulnerable and secure Korean public-system code compared with exact control checkpoints. | Code transforms with reason, test, and policy evidence. | Parent: Public Development; links to KISA Compliance; CTA: View secure coding workflow. |
| 108 | `/korean-public-api`<br>**Korean Public API Engineering** | Supports secure integration patterns, schemas, authentication, logging, error handling, and public-data APIs. | API lifecycle; specification; client/server generation; validation; audit; rate and privacy controls. | Government service APIs connected through a controlled integration gateway and schema registry. | API request flows through validation and audit stages. | Parent: Public Development; links to Government Documentation; CTA: Review API support. |
| 109 | `/government-java`<br>**Java for Public Systems** | Covers Spring, legacy Java, application servers, transactions, batch, testing, and secure modernization. | Technology map; supported code tasks; legacy compatibility; quality gates. | Public-sector Java application architecture spanning web, service, persistence, batch, and integration. | Legacy and modern paths highlight shared verification controls. | Parent: Public Development; links to Spring, MyBatis, Batch; CTA: Explore Java capabilities. |
| 110 | `/government-database`<br>**Government Database Engineering** | Supports Oracle, PostgreSQL, MariaDB, schema review, migrations, SQL safety, performance, and PII access. | Database matrix; query analysis; migration approval; audit; synthetic test data. | Database topology with PII-classified tables, safe query paths, and migration approvals. | Query path highlights access and audit controls. | Parent: Public Development; links to MyBatis and PII; CTA: Review database workflow. |
| 111 | `/government-devsecops`<br>**Public-Sector DevSecOps** | Integrates Git, CI/CD, artifact repositories, approvals, SBOM, provenance, and release evidence. | Pipeline stages; GongCode checks; protected release; evidence package. | Secure public-sector pipeline from commit through build, scan, approval, signing, and deployment candidate. | Pipeline activates each gate; 3-minute walkthrough. | Parent: Public Development; links to Connect and Supply Chain; CTA: Map your pipeline. |
| 112 | `/government-accessibility`<br>**Accessible Public Software** | Helps developers identify and remediate accessibility issues with human verification. | Accessibility checks; keyboard; semantics; Korean content; evidence; limitations. | Accessible government service interface with keyboard focus, semantic structure, captions, and contrast callouts. | Keyboard-only navigation demo; reduced-motion comparison. | Parent: Public Development; links to Website Accessibility docs; CTA: Review accessibility workflow. |
| 113 | `/government-documentation`<br>**Government-Grade Technical Documentation** | Generates controlled architecture, API, operations, change, test, and audit documentation from evidence. | Document types; source trace; approvals; HWP/PDF export strategy; version control. | Technical document set linked to source code, tests, policies, and release evidence. | Document sections populate from verified evidence, not unsupported claims. | Parent: Public Development; links to Provenance and Evidence; CTA: See documentation outputs. |
| 114 | `/government-evaluation`<br>**Korean Government Coding Benchmarks** | Describes GongCode evaluation packs for Korean, eGovFrame, secure coding, PII, injection, and legacy systems. | Benchmark catalog; dataset governance; scorecard; approval threshold; regression. | Evaluation lab with Korean code tasks and security scenarios across multiple models. | Scorecards build into an approval matrix. | Parent: Public Development; links to Evaluate and Model Approval; CTA: Review benchmark design. |

### I. Solutions (8 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 115 | `/solutions/ministry`<br>**GongCode for Central Government Ministries** | Positions sovereign AI engineering for ministry application portfolios, shared standards, and audit obligations. | Ministry operating model; central policies; project onboarding; shared GPU; evidence. | Large ministry application portfolio managed through one governed AI engineering control plane. | Projects onboard into a common policy baseline. | Parent: Solutions; links to Sovereign, Compliance, Control; CTA: Plan ministry deployment. |
| 116 | `/solutions/local-government`<br>**GongCode for Local Government** | Supports municipalities with constrained staff, legacy systems, shared services, and strong privacy needs. | Local-government challenges; deployment options; legacy modernization; managed operations. | Municipal digital services and legacy systems connected to a secure shared GongCode environment. | Service modules modernize in phased waves. | Parent: Solutions; links to Legacy Migration and Private Deployment; CTA: Review local-government model. |
| 117 | `/solutions/public-enterprise`<br>**GongCode for Public Enterprises** | Combines government-grade governance with enterprise-scale developer productivity and hybrid infrastructure. | Business and public obligations; multi-team control; private GPU; DevSecOps; provenance. | Public enterprise engineering teams using private GPU and shared governance across many repositories. | Team usage aggregates into governance and capacity views. | Parent: Solutions; links to Hybrid, DevSecOps, GPU; CTA: Design enterprise rollout. |
| 118 | `/solutions/defense-adjacent`<br>**GongCode for Highly Restricted Engineering** | Describes dedicated, air-gapped, least-privilege patterns for defense-adjacent and highly sensitive work without claiming suitability for classified systems by default. | Strict disclaimer; dedicated zones; no network; dedicated models/GPUs; evidence and operations. | Highly restricted isolated engineering enclave with dedicated execution, model, and evidence zones. | No cross-zone flows; signed offline update only. | Parent: Solutions; links to Air-Gapped and MicroVM; CTA: Request restricted architecture review. |
| 119 | `/solutions/critical-infrastructure`<br>**GongCode for Critical Infrastructure** | Supports controlled development for energy, transport, communications, and other essential systems. | Safety and availability risks; change approval; simulation; incident response; DR. | Critical infrastructure software control room with separated test environment and strict change gates. | Change moves through simulation and approval before export. | Parent: Solutions; links to HA/DR, Incident Response; CTA: Review critical-system controls. |
| 120 | `/solutions/system-integrator`<br>**GongCode for Government System Integrators** | Standardizes secure AI-assisted delivery across customers while preserving tenant and evidence separation. | Customer workspaces; reusable policy packs; delivery workflow; evidence handoff; licensing. | System integrator managing multiple strictly separated government projects from one operational layer. | Tenant boundaries remain fixed as shared templates propagate. | Parent: Solutions; links to SI Edition and Custom Packs; CTA: Explore SI program. |
| 121 | `/solutions/regulatory-agency`<br>**GongCode for Regulatory Agencies** | Provides traceability, privacy, rule-grounded engineering, and audit evidence for regulator-operated systems. | Rule changes; impact analysis; provenance; privacy; release evidence. | Regulatory rule change linked to affected code, tests, approvals, and release evidence. | Rule-to-code impact graph animates. | Parent: Solutions; links to Trace and Audit Readiness; CTA: Follow a regulatory change. |
| 122 | `/solutions/education-research`<br>**GongCode for Public Research and Education** | Supports controlled experimentation, shared GPU, model evaluation, and research reproducibility. | Research workspaces; model registry; GPU quotas; dataset controls; reproducibility. | Public research lab with governed model experiments, shared GPU scheduling, and reproducible evidence. | Experiment runs become signed evaluation records. | Parent: Solutions; links to Evaluate, GPU, Replay; CTA: Design a research environment. |

### J. Trust, Resources & Company (6 pages)

| # | URL and title | Description | Rough design | AI image brief | Animation/video | Navigation and CTA |
|---:|---|---|---|---|---|---|
| 123 | `/trust-center`<br>**GongCode Trust Center** | Publishes security architecture, release integrity, control status, certifications, subprocessors, and disclosure channels. | Trust summary; verified claims; documents; status; update history; request-access workflow. | Clean technical trust center with security documents, signed releases, and verification status. | Document freshness indicators update; no decorative animation. | Parent: Resources; links to Security Advisories and Compliance; CTA: Request assurance package. |
| 124 | `/security-advisories`<br>**Security Advisories** | Publishes GongCode vulnerabilities, affected versions, mitigations, patches, and signatures. | Advisory list; severity; affected editions; verification; offline update instructions. | Minimal security advisory interface with version and signature details. | Filter and version comparison only; no hero animation. | Parent: Trust Center; links to Update; CTA: Subscribe to advisory notices. |
| 125 | `/resources`<br>**GongCode Resources** | Hub for architecture papers, checklists, videos, diagrams, procurement guides, and technical articles. | Filtered resource library; featured architecture paper; topic collections; language toggle. | Organized technical resource library with diagrams, videos, and government engineering guides. | Cards filter smoothly; video previews respect reduced motion. | Parent: Resources; links to Docs and Trust Center; CTA: Download architecture brief. |
| 126 | `/docs`<br>**GongCode Documentation** | Public documentation entry for architecture, APIs, deployment, integrations, policy, and operations. | Docs search; version selector; product tree; quickstarts; reference architecture. | Developer documentation interface with Korean code, diagrams, and version badges. | Interactive code samples; no auto-playing video. | Parent: Resources; links to all technical sections; CTA: Open documentation. |
| 127 | `/company`<br>**About GongCode** | Explains mission, product principles, team disciplines, and commitment to Korean public-sector software. | Mission; principles; product history; leadership functions; careers; contact. | Abstract image of Korean software engineering, secure infrastructure, and public service working together. | Subtle timeline reveal; no fabricated customer logos. | Parent: Company; links to Contact and Trust; CTA: Meet the team. |
| 128 | `/contact`<br>**Request a GongCode Technical Briefing** | Routes procurement, architecture, security, partnership, and support inquiries without exposing product systems. | Inquiry type; organization; deployment interest; data-minimized form; response process. | Professional briefing room with GongCode architecture on a display, no identifiable people. | Form-step transitions; no background video. | Parent: Company; links to Trust and Architecture; CTA: Submit briefing request. |

## 32. GongCode Control Reference Layout

```text
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│ GONGCODE CONTROL          기관: 전체 ▼  등급: 전체 ▼  시간: LIVE  ● 정상 ● 경고 ● 위험   │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ Overview | Live Harnesses | Security | Prompts | Policy | Provenance | Models | GPU | Audit │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ ACTIVE HARNESS 184 │ ACTIVE USERS 127 │ RUNNING 243 │ GPU 78% │ HIGH ALERTS 3 │ BLOCKED 89 │
├──────────────────────────────────────────────┬───────────────────────────────────────────────┤
│ LIVE ACTIVITY                                │ SECURITY ALERTS                               │
│ 04:16:02 MOIS-021 read JwtProvider.java      │ 🔴 Credential-file access blocked            │
│ 04:16:01 NTS-014 running unit tests          │    Session a81d2c / Policy GC-SEC-014         │
│ 04:15:59 SEOUL-008 prompt → approved model   │    [Investigate] [Isolate] [Capture evidence] │
│ 04:15:57 DAPA-004 network request denied     │ 🟠 Unapproved package requested               │
├──────────────────────────────────────────────┼───────────────────────────────────────────────┤
│ TOKENS AND LATENCY                           │ GPU FLEET                                     │
│ Input 1.16B | Output 260M | TTFT 1.7 sec     │ H100-0 91% 72/80 GB 71°C OK                  │
│ Queue 38 | Active 184 | Errors 0.8%          │ H100-1 99% 79/80 GB 79°C HOT                 │
├──────────────────────────────────────────────┴───────────────────────────────────────────────┤
│ POLICY: 18,421 allow | 324 approval | 89 blocked | 12 escalated                           │
│ Top reasons: secrets 34% | network 27% | file permission 18% | unsafe command 11%         │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

The dashboard must support wall-display mode, keyboard operation, high-contrast mode, and a privacy mode that hides prompt text while retaining operational metadata.

---

## 33. Implementation Roadmap

The scope is too large for a single undifferentiated implementation project. It should be managed as twelve coordinated workstreams with common contracts and quarterly integration gates.

### 33.1 Phase 0 — Product and Security Foundation (Weeks 0–6)

**Deliverables**

- Confirm product editions and initial target deployment.
- Complete threat model.
- Approve service boundaries and contracts.
- Establish monorepo and release signing.
- Implement organization, user, project, repository, session, action, and decision schemas.
- Establish identity integration and mTLS.
- Establish event bus, control database, telemetry, and evidence prototype.
- Define mandatory baseline policy.
- Build a non-production model Gateway proof of concept.
- Build a microVM feasibility prototype.
- Create initial public design system and website content model.
- Register brand, domain, and security contacts through normal business processes.

**Gate**

A user can create an identity-bound session, send one governed model request, receive one response, and verify a signed audit chain. No code execution is permitted yet.

### 33.2 Phase 1 — Secure Harness and Disposable Runtime MVP (Months 2–4)

**Deliverables**

- CLI Harness with Ask, Plan, Edit, Test, and Review modes.
- Git broker and immutable snapshots.
- MicroVM sandbox launch and destruction.
- File, Command, Network, Resource, Logging, and Runtime Boxes.
- No-network default.
- Read/write scope.
- Approved build and test commands.
- Candidate patch export.
- Basic Control session list and timeline.
- Basic evidence bundle.
- Initial secret and PII detection.
- vLLM or SGLang endpoint through Gateway.
- Initial 12-page website launch subset.

**Gate**

A developer can safely edit and test a Java repository inside a disposable sandbox. A compromised mock model cannot read denied files, open the internet, access host sockets, or export an unscanned artifact.

### 33.3 Phase 2 — Guard and Control Operations (Months 4–7)

**Deliverables**

- Full Assurance Box framework and common decision contract.
- Prompt, Context, Secrets, PII, Injection, Package, License, Crypto, Model, Response, Output, and Evidence Boxes.
- Control Overview, Live Harnesses, Session Inspector, Security Operations, Sandbox Fleet, and GPU views.
- Incident containment and forensic snapshot.
- Policy Studio simple settings.
- Approval service.
- Internal package mirror flow.
- SBOM and vulnerability scans.
- Prometheus/OpenTelemetry operational baseline.
- 40-page website release.

**Gate**

Security operators can detect, investigate, contain, and evidence simulated secret, PII, prompt-injection, package, network, and sandbox incidents.

### 33.4 Phase 3 — Provenance and Model Governance (Months 6–9)

**Deliverables**

- Provenance event model.
- File, hunk, AST, and semantic span mapping.
- Git commit trailers and notes.
- Provenance Explorer.
- Model Registry.
- Model intake and approval workflow.
- Evaluate scorecards and first Korean benchmark packs.
- Model canary and rollback.
- Prompt Governance workspace.
- Signed model and policy artifacts.
- 72-page website release.

**Gate**

An authorized reviewer can click AI-assisted code and verify user, model, time, commit, context, tools, policies, tests, approvals, and evidence. The record survives a controlled file rename and refactor.

### 33.5 Phase 4 — Korean Government Intelligence and Compliance Center (Months 8–12)

**Deliverables**

- eGovFrame 5.0 retrieval corpus and evaluation.
- Korean secure-coding policy mappings.
- Korean PII benchmark.
- Public-sector Java, MyBatis, SQL, batch, and legacy evaluation.
- Compliance Center.
- KISA/MOIS, CSAP, ISMS-P, PIPA, AI Basic Act, and KCMVP-aware profiles.
- Policy-as-code editing, simulation, signature, and staged rollout.
- Evidence freshness and audit exports.
- Eclipse/eGovFrame extension beta.
- 100-page website release.

**Gate**

The platform generates a control-to-evidence matrix that clearly distinguishes product controls, organizational procedures, and external certification. A government Java task passes the agreed Korean benchmark and security thresholds.

### 33.6 Phase 5 — Production Hardening and Air-Gap Operations (Months 10–15)

**Deliverables**

- HA control plane.
- Backup and restore.
- DR exercises.
- Signed offline update bundles.
- Air-gap quarantine and rollout rings.
- Multi-node GPU capacity and routing.
- Dedicated and partitioned GPU profiles.
- Tenant isolation tests.
- Full security red team.
- Independent penetration test.
- Operational runbooks.
- Support evidence export.
- 128-page public site completion.
- Trust Center and Security Advisories.

**Gate**

A representative customer environment can install, update, operate, restore, and roll back GongCode without public internet access. Independent testing finds no unresolved critical issue.

### 33.7 Phase 6 — Certification and Scaled Operations (Months 13–18)

**Deliverables**

- Separate CSAP-cloud deployment scope and operating model.
- Certification evidence and process readiness.
- Multi-agency capacity and quota management.
- SI edition and customer evidence handoff.
- Advanced provenance survival.
- Windows sandbox pool where required.
- Advanced model evaluation and red-team automation.
- Production SLO reporting.
- Policy-pack marketplace limited to signed approved publishers.
- Korean and English documentation completion.

**Gate**

Each marketed edition has a documented architecture, control boundary, operations model, support model, evidence model, and truthful certification status.

---

## 34. Workstream Backlog and Acceptance Criteria

### Workstream 1: Identity, Tenancy, and Sessions

**Build**

- Organization, agency, tenant, user, role, device, project, and repository lifecycle.
- SSO, MFA, certificates, and workload identity.
- RBAC plus ABAC.
- JIT and break-glass.
- Session manifests and capability tokens.

**Acceptance**

- No action without actor and workload identity.
- Cross-tenant tests fail.
- Privilege elevation expires.
- Every administrative permission change is signed and searchable.

### Workstream 2: Harness

**Build**

- CLI.
- VS Code extension.
- Eclipse/eGovFrame extension.
- Task planning.
- context preview.
- tools.
- diff and patch review.
- Korean UX.

**Acceptance**

- Same policy behavior across interfaces.
- Harness cannot create its own capabilities.
- User can understand why an action is denied.
- Low-risk flow remains usable without repeated redundant approvals.

### Workstream 3: Runtime

**Build**

- Scheduler.
- microVM images.
- rootless tool runner.
- filesystem.
- network isolation.
- nested execution.
- destruction.
- Windows pool later.

**Acceptance**

- Sandbox escape test suite passes.
- No host sockets or credentials.
- Network is externally denied.
- Workspace cannot be recovered after key destruction under the defined threat model.

### Workstream 4: Guard

**Build**

- All Assurance Boxes.
- common action and decision contracts.
- policy obligations.
- transformations.
- blocking and escalation.
- deterministic test suites.

**Acceptance**

- Every box has unit, contract, integration, and bypass tests.
- Policy outage behavior is defined and tested.
- Decision explanations reference exact rule and source.

### Workstream 5: Model and GPU

**Build**

- Gateway.
- Registry.
- approval.
- evaluation.
- endpoints.
- scheduler.
- telemetry.
- routing.
- rollback.

**Acceptance**

- Only approved hash and endpoint combinations receive requests.
- Suspension stops new traffic.
- GPU operators cannot inspect prompts.
- Capacity tests use real workload profiles.

### Workstream 6: Control

**Build**

- all workspaces.
- real-time streams.
- search.
- investigations.
- policy settings.
- compliance.
- role-specific views.
- reports.

**Acceptance**

- Critical incident containment within three clicks.
- Sensitive prompt access is independently permissioned.
- Every admin action is audited.
- Accessibility and Korean usability tests pass.

### Workstream 7: Trace and Evidence

**Build**

- event correlation.
- provenance spans.
- Git integration.
- replay.
- evidence bundles.
- verification.
- retention.

**Acceptance**

- Line-level trace is complete for protected merges.
- Missing provenance blocks protected export where configured.
- Evidence bundle verifies offline.
- Replay reports all version divergence.

### Workstream 8: Korean Government Intelligence

**Build**

- corpora.
- SFT datasets.
- retrieval.
- policy mappings.
- evaluation packs.
- model cards.

**Acceptance**

- Current documents are versioned and cited.
- Mutable rules are not only stored in weights.
- Dataset licenses and lineage are complete.
- Customer code is excluded from training by default.

### Workstream 9: Integrations

**Build**

- Git.
- CI/CD.
- package.
- artifacts.
- identity.
- SIEM.
- documents.
- ticketing.

**Acceptance**

- Every integration uses scoped service identity.
- Connector outage and retry behavior are tested.
- No connector grants broader access than the user and task.

### Workstream 10: Offline Updates

**Build**

- bundle builder.
- signatures.
- media process.
- quarantine.
- canary.
- rollback.
- delta distribution.

**Acceptance**

- One-byte bundle modification is detected.
- Expired or revoked signing key is rejected.
- Rollback is rehearsed.
- Import evidence is complete.

### Workstream 11: Website

**Build**

- public design system.
- CMS/content-as-code.
- 128 page templates and content.
- diagrams.
- generated imagery.
- product videos.
- docs.
- Trust Center.
- search.
- analytics with privacy controls.

**Acceptance**

- All 128 paths have distinct content.
- No thin duplicate pages.
- Every page has title, description, design, media, navigation, and CTA from this plan.
- Accessibility, performance, security headers, SEO, Korean copy, and legal review pass.
- Public hosting has no route or credentials for product systems.

### Workstream 12: Certification and Operations

**Build**

- operating procedures.
- asset inventory.
- control mapping.
- evidence schedule.
- personnel roles.
- incident, DR, change, and vulnerability programs.

**Acceptance**

- Product controls and organization processes are clearly separated.
- Certification claims match actual scope and status.
- Evidence is reproducible without screenshots as the primary source.
- Annual and event-driven review schedules are operational.

---

## 35. Website Production Plan

### 35.1 Content waves

| Wave | Pages | Objective |
|---|---:|---|
| Wave 1 | 12 | Brand, platform, Harness, Control, Security, Closed Network, Contact |
| Wave 2 | 28 additional | Core products, Assurance Boxes, sandbox, GPU, policy |
| Wave 3 | 32 additional | Provenance, governance, compliance, government engineering |
| Wave 4 | 28 additional | Solutions, deep infrastructure, model intelligence |
| Wave 5 | 28 additional | Complete 128 pages, Trust Center, advisories, docs, resources |

### 35.2 Asset production

Create:

- 25 architecture diagrams,
- 20 UI mockups,
- 15 short product loops,
- 8 narrated product videos,
- 12 security control animations,
- 10 provenance interactions,
- 8 infrastructure illustrations,
- 8 government engineering illustrations,
- 10 solution diagrams,
- downloadable reference architecture packs.

All generated assets need:

- prompt,
- model/tool used,
- generation date,
- source assets,
- editor,
- rights status,
- accessibility alt text,
- approval,
- final checksum.

GongCode should use its own provenance system for website code and AI-generated website assets as an internal demonstration.

### 35.3 Public website technical architecture

Recommended:

- Next.js or equivalent static-capable framework.
- Content stored in version-controlled MDX or structured content repository.
- Korean and English locale routing.
- Pre-render core pages.
- Content Security Policy.
- Strict security headers.
- Signed builds.
- SBOM.
- image optimization.
- accessible SVG diagrams.
- privacy-minimized analytics.
- isolated form service.
- no third-party chat widget by default.
- no public model endpoint.
- separate status and advisory publishing process.

### 35.4 Content governance

Each page has:

- product owner,
- technical reviewer,
- security reviewer,
- compliance reviewer when applicable,
- Korean copy editor,
- English copy editor,
- source date,
- next review date,
- factual claims register,
- asset provenance,
- publication approval.

Pages about official frameworks or rules must be reviewed after source changes. A “current as of” banner is required for detailed compliance pages.

### 35.5 Animation principles

- Motion explains state transitions, not decoration.
- Never imply a blocked control after the model already received sensitive data.
- Diagrams must accurately show enforcement order.
- Avoid continuous flashing or high-frequency motion.
- Provide pause.
- Provide static alternative.
- Do not autoplay narrated audio.
- Product demos use synthetic data.
- Security alerts must not contain realistic active credentials or resident numbers.

---

## 36. Team Plan

### 36.1 Core product team at full build

| Function | Suggested staffing |
|---|---:|
| Product leadership | 2 |
| Security architecture and application security | 4–6 |
| Harness and developer experience | 5–7 |
| Runtime/sandbox and endpoint agents | 5–7 |
| Backend control services | 7–10 |
| Control web and design system | 5–7 |
| Model serving and GPU platform | 4–6 |
| Model evaluation/data/fine-tuning | 5–8 |
| Provenance and evidence | 4–6 |
| DevOps/SRE/air-gap release | 5–7 |
| Compliance, privacy, and audit | 3–5 |
| QA, security test, and red team | 5–7 |
| Technical writing and website | 5–8 |
| Solutions architecture and customer deployment | 4–6 |

A smaller initial team can deliver the Phase 1 MVP, but the full platform cannot be built safely by treating security, GPU infrastructure, provenance, compliance, and website content as side tasks.

### 36.2 Required named owners

- Chief Product Owner.
- Chief Security Architect.
- Runtime Isolation Owner.
- Model Risk Owner.
- Privacy Owner.
- Compliance Evidence Owner.
- Release Signing Owner.
- Incident Response Owner.
- Website Claims Owner.
- Data and Training Governance Owner.

---

## 37. Procurement and Packaging

### 37.1 Commercial packages

- **GongCode Department:** single organization, moderate GPU, standard policies.
- **GongCode Agency:** HA, multiple projects, advanced Control, evidence, dedicated policy packs.
- **GongCode Sovereign:** air-gapped, offline updates, customer-owned models/GPUs/keys.
- **GongCode CSAP Cloud:** separately certified cloud service when certification is achieved.
- **GongCode SI:** multi-customer delivery and evidence handoff.
- **GongCode Evaluate Add-on:** advanced model and agent evaluation.
- **GongCode Managed Operations:** customer-approved operational service.

### 37.2 Billable services

- architecture assessment,
- policy-pack development,
- eGovFrame and legacy corpus onboarding,
- model evaluation,
- private model fine-tuning,
- GPU sizing and benchmark,
- air-gap installation,
- integration,
- compliance evidence preparation,
- security testing,
- administrator and developer training.

---

## 38. Key Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Scope becomes too broad | Delayed, incoherent platform | Phase gates, stable contracts, one unified Control UI |
| Security boxes become high-latency | Developers bypass system | parallel checks, caching of safe metadata, risk-tier paths |
| Provenance is inaccurate after refactor | Loss of trust | AST/semantic mapping, ambiguity workflow, confidence indicators |
| Compliance claims overreach | Legal and reputational risk | claims register, review, certification-scope disclaimer |
| Model quality disappoints | Low adoption | multiple approved models, evaluations, retrieval, task routing |
| GPU capacity is underestimated | Queues and poor UX | workload benchmarks, admission control, capacity reservation |
| Air-gap updates are cumbersome | Stale vulnerable deployments | signed delta bundles, clear rings, automated verification |
| PII scanner false positives | Friction | Korean benchmark, tokenization, scoped override with evidence |
| PII scanner false negatives | Leakage | layered detection, DLP at multiple points, red-team data |
| Prompt logs become surveillance | Trust and privacy harm | minimization, permissioned access, metadata-first operations |
| Policy complexity overwhelms admins | Misconfiguration | simple builder, profiles, simulation, inheritance explanation |
| Third-party licenses conflict | Product or customer risk | SBOM, legal review, License Box, edition-specific validation |
| Public website becomes attack bridge | Product compromise | total environment separation |
| Customer requests direct production agent | High systemic risk | prohibit baseline; separate approved future workflow if justified |
| Model vendor or project changes license | Deployment uncertainty | registry restrictions, multiple models, artifact-level approval |

---

## 39. Launch Readiness Checklist

### Product

- Harness task flow is stable.
- Control covers operations and security.
- Sandbox isolation is independently tested.
- Model approval is enforced.
- Provenance is complete for protected code.
- Evidence verifies offline.
- Update and rollback are rehearsed.
- Korean documentation is complete.

### Security

- threat model current,
- independent penetration test complete,
- critical/high findings resolved or formally risk-accepted,
- keys and signing separated,
- incident response exercised,
- break-glass tested,
- audit tampering tests passed,
- supply-chain inventory complete.

### Governance

- model inventory and approvals current,
- policy source and precedence visible,
- prompt retention approved,
- privacy impact reviewed,
- responsible-AI operating roles assigned,
- exceptions have expiry,
- employee monitoring safeguards documented.

### Public claims

- certification claims verified,
- framework versions current,
- benchmark methodology published,
- no fabricated customers,
- no government endorsement implication,
- diagrams match deployed architecture,
- security claims map to tests.

### Website

- all 128 pages complete,
- Korean and English quality reviewed,
- accessibility tested,
- performance budget passed,
- forms isolated,
- security headers deployed,
- Trust Center live,
- advisory process operational,
- page review dates scheduled.

---

## 40. Definition of Done

GongCode is not complete merely when a model can edit code. The platform reaches the intended definition of done when:

1. A government developer can use a high-quality Korean coding agent inside an approved closed environment.
2. The agent cannot cross file, command, network, credential, package, model, runtime, or export boundaries.
3. Administrators can see and control live sessions, tokens, alerts, models, GPUs, policies, and incidents.
4. An auditor can click an AI-assisted code block and verify its complete grounded provenance.
5. The organization can select and customize compliance and engineering profiles without changing product code.
6. Models, policies, packages, tools, images, and updates all have approval, provenance, signature, and expiry.
7. Personal information and secrets are controlled before model input and at every output path.
8. Sandboxes are disposable and independently monitored.
9. Evidence is produced continuously and verifies offline.
10. The public website explains the system with at least 128 substantive pages, accurate diagrams, accessible media, and truthful claims.

---

## 41. Official Source Baseline Used for This Plan

The detailed control mappings must be created from the official full documents. This plan used the following current source baseline for architecture and backlog decisions:

1. **eGovFrame official development guide, version 5.0.** The official guide describes development lifecycle support including implementation, testing, configuration/change management, and deployment tools.
2. **MOIS Software Development Security Guide and KISA Software Weakness Diagnostic Guide.** The published guidance supports secure development and weakness detection for administrative and public information systems.
3. **행정기관 및 공공기관 정보시스템 구축·운영 지침.** Current published version shown as MOIS Notice No. 2025-1, effective 2025-01-02.
4. **KISA Cloud Security Assurance Program materials.** The official CSAP introduction describes IaaS, SaaS, and DaaS types; current and grade-based control structures; certification and follow-up evaluation.
5. **ISMS-P official control structure.** It separates management-system establishment and operation, protection requirements, and personal-information lifecycle requirements.
6. **개인정보의 안전성 확보조치 기준.** PIPC Notice No. 2026-9, effective 2026-07-01, including access control, encryption, access-record, public-system, and disaster-related requirements.
7. **Korean AI Basic Act and Enforcement Decree.** Current official text is effective 2026-07-21 and includes AI lifecycle risk and safety-related obligations for applicable systems, as well as impact-assessment considerations for high-impact AI.
8. **KCMVP official program information.** The program addresses validation of cryptographic modules introduced into national and public networks where applicable.

Before implementation or publication, assign a compliance owner to obtain the complete official texts, record checksums and effective dates, identify amendments, and produce reviewed control interpretations. Search snippets and summaries are not sufficient evidence for certification.

---

## 42. Immediate Next Actions

1. Approve GongCode product suite and Assurance Box names.
2. Select the first deployment target: sovereign on-premises pilot is recommended before CSAP-cloud scope.
3. Select one representative Java/eGovFrame repository using synthetic or authorized code.
4. Select one initial coding model and one fallback model for evaluation.
5. Build the Phase 0 identity → session → policy → gateway → audit vertical slice.
6. Prototype a microVM session with no network and patch-only export.
7. Define the first 25 mandatory policy rules.
8. Define provenance manifest version 1 before large-scale Harness implementation.
9. Build the first Korean evaluation set before fine-tuning.
10. Launch the first 12 public pages only after architecture and claims review.
