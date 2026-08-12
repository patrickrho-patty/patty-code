package i18n

import (
	"os"
	"strings"
)

type Messages struct {
	WelcomeTitleFmt string // first-run box title — %s = product name (styled)
	NoConfigYet     string // first-run cue under the welcome box

	InitHint string

	ChatTip                   string // tip line under the chat banner
	ChatComposerTitle         string // heading above the borderless composer
	ChatComposerInputTitle    string // title on the bordered text input
	ChatComposerPlaceholder   string // default empty-composer prompt
	ChatComposerCommandsHint  string // slash palette hint below the composer
	ChatComposerFilesHint     string // file reference hint below the composer
	ChatComposerShellHint     string // shell mode hint below the composer
	ChatComposerShortcutsHint string // shortcut help hint below the composer
	YoloModePrompt            string // YOLO activation confirmation prompt
	YoloModeDisclaimer        string // YOLO no-approval disclaimer body
	YoloApproveLabel          string // approve-YOLO-for-this-folder action label
	ChatModeAutoLabel         string // current-mode label: auto
	ChatModePlanLabel         string // current-mode label: plan
	ChatMastheadTitle         string // brand line below the launch artwork
	ChatUserLabel             string // transcript timeline label for the human turn
	ChatPlanLabel             string // transcript timeline modifier for a plan-mode turn
	TurnCancelled             string // shown when Ctrl-C aborts the in-flight turn but the chat keeps running
	InterruptedRecovery       string // replay notice for a durable interrupted turn
	RecoveryPaused            string // controlled Auto retry pause; user can continue in the next message
	NoSessionToResume         string // shown when --continue / --resume finds nothing
	ResumeRequiresTTY         string // shown when --resume runs piped instead of on a terminal
	PickSessionLabel          string // header on the --resume picker

	ResumeBusy          string // shown when /resume is used mid-turn
	ResumeBadIndexFmt   string // shown when /resume gets an out-of-range index (one %d)
	ResumeAlreadyActive string // shown when /resume targets the current session
	ResumedTitle        string // banner title after a /resume switch

	RenameUsage            string // /rename with no args
	RenameNoSession        string // /rename with no active session
	RenameDoneFmt          string // /rename succeeded (one %s = new title)
	ResumePickTitle        string // header in the interactive resume picker
	ResumePickHint         string // keyboard hint in the interactive resume picker
	ResumeRecoveryBadgeFmt string // recovery-copy badge — %s = short parent session id

	ChatThinking                           string // live reasoning marker label, e.g. "thinking…"
	ChatThoughtForFmt                      string // collapsed reasoning summary, "%d" = elapsed s
	ChatStatusThinkingFmt                  string // "%s thinking… (%ds · <cancel hint>)" — %s = spinner, %d = elapsed s
	ChatToolWorkingFmt                     string // "%s working · %ds" under a running tool — %s = spinner, %d = elapsed s
	ChatSubagentPhaseQueued                string // sub-agent progress phase label ("queued")
	ChatSubagentPhaseRunning               string // ("running")
	ChatSubagentPhaseReasoning             string // ("reasoning")
	ChatSubagentPhaseResponding            string // ("responding")
	ChatSubagentPhaseTool                  string // ("using tools")
	ChatSubagentPhaseRetrying              string // ("retrying")
	ChatSubagentPhaseCompleted             string // ("completed")
	ChatSubagentPhaseFailed                string // ("failed")
	ChatSubagentPhaseCancelled             string // ("cancelled")
	ChatSubagentProgressFmt                string // live progress line — %s = phase label, %d = elapsed s, %d = idle s ("%s · %ds · %ds ago")
	ChatSubagentProgressDoneFmt            string // terminal summary — %s = phase label, %d = duration s ("%s · %ds")
	ChatSubagentPreviewLabel               string // verbose preview marker ("▎")
	ChatStatusRetryingFmt                  string // "%s retrying (%d/%d)…" — %s = spinner, %d/%d = attempt/max
	ChatStatusCancellingFmt                string // "%s stopping… (%ds · Ctrl+C exits)" — %s = spinner, %d = elapsed s
	ChatStatusIdle                         string // shortcuts hint when idle
	ChatStatusYoloIdle                     string // shortcuts hint when idle in YOLO/bypass mode
	ChatStatusCycleHintCompact             string // readable shortcut hint used by the persistent footer
	ChatTurnReceiptLabel                   string // compact per-turn usage receipt attached to the completed assistant response
	ChatTurnReceiptIn                      string // prompt token input marker in the turn receipt
	ChatTurnReceiptCached                  string // cache-hit token marker in the turn receipt
	ChatTurnReceiptNew                     string // fresh (cache-miss) token marker in the turn receipt
	ChatTurnReceiptOut                     string // completion token output marker in the turn receipt
	ChatTurnReceiptReasoning               string // reasoning token marker in the turn receipt
	ChatTurnReceiptEstimated               string // estimated-usage marker in the turn receipt
	ChatTurnReceiptPrefixChanged           string // cache prefix invalidation warning in the turn receipt
	ChatTurnReceiptUnknownReason           string // fallback reason code when the prefix change has none
	ChatStatusModelLabel                   string
	ChatStatusEffortLabel                  string
	ChatStatusHeadroomLabel                string
	ChatStatusVersionLabel                 string
	ChatStatusCacheLabel                   string
	ChatStatusContextLabel                 string
	ChatStatusCompactLabel                 string
	ChatStatusJobsLabel                    string
	ChatStatusBalanceLabel                 string
	ChatStatusCacheNowFmt                  string // cache status tag, "%s" = latest-turn hit rate with percent sign
	ChatStatusCacheAvgFmt                  string // cache status tag, "%s" = session-average hit rate with percent sign
	ChatEffortAuto                         string
	ChatEffortLow                          string
	ChatEffortMedium                       string
	ChatEffortHigh                         string
	ChatEffortXHigh                        string
	ChatEffortMax                          string
	ChatStatusPlanApproval                 string // shortcuts hint while a plan is pending
	PlanApprovalPrompt                     string // one-line "plan above is ready" banner shown above the input
	PlanApprovalChoices                    string // start / revise / exit-without-executing choice list
	ChatStatusToolApproval                 string // shortcuts hint while a tool call awaits approval
	ToolApprovalPromptFmt                  string // approval banner — tool, subject suffix, source/intent detail, choices
	ToolApprovalChoices                    string // standard approval choice list
	BashPrefixChoices                      string // approval choice list when a bash prefix can be granted
	PlanModeReadOnlyCommandChoices         string // approval choice list for plan-mode read-only command trust
	FreshHumanApprovalChoices              string // approval choice list for prompts that cannot be remembered
	RecoveryApprovalChoices                string // one-shot Auto Guard decision list
	RecoveryPlanChangeChoices              string // material Auto plan transition decision list
	RecoveryPlanDecisionPrompt             string // neutral title for a material Auto plan transition
	RecoveryPlanBeforeFmt                  string // compact previous-plan line, one %s
	RecoveryPlanAfterFmt                   string // compact proposed-plan line, one %s
	RecoveryTaskGrantChoices               string // Auto Guard list with a current-task semantic grant
	SandboxEscapeApprovalChoices           string // approval choice list for OS sandbox escape prompts
	ApprovalNeededFmt                      string // notification text for a pending approval, tool only
	ApprovalNeededWithSubjectFmt           string // notification text for a pending approval with subject
	ToolApprovalSourceFmt                  string // "Source: %s" / "출처: %s"
	ToolApprovalBuiltIn                    string // built-in tool source label
	ToolApprovalImageUse                   string // image-understanding detail for understand_image-style tools
	ApprovalToolLabelBash                  string // user-facing label for bash approvals
	ApprovalToolLabelEditFile              string // user-facing label for edit_file approvals
	ApprovalToolLabelWriteFile             string // user-facing label for write_file approvals
	ApprovalToolLabelMultiEdit             string // user-facing label for multi_edit approvals
	ApprovalToolLabelMoveFile              string // user-facing label for move_file approvals
	ApprovalToolLabelWebFetch              string // user-facing label for web_fetch approvals
	ApprovalToolLabelRunSkill              string // user-facing label for run_skill approvals
	ApprovalToolLabelRemember              string // user-facing label for remember approvals
	ApprovalToolLabelForget                string // user-facing label for forget approvals
	ApprovalToolLabelSandboxEscape         string // user-facing label for OS sandbox escape approvals
	ApprovalToolLabelPlanModeReadOnly      string // user-facing label for plan-mode read-only command trust approvals
	MemoryApprovalSaveUpdate               string // subject prefix for remember approval
	MemoryApprovalBodyLabel                string // label before the body excerpt in remember approval
	MemoryApprovalArchiveFmt               string // subject for forget approval, %q = memory name
	PlanModeBashTrustSubjectFmt            string // subject for bash read-only prefix trust approval, prefix + command
	PlanModeBashTrustReason                string // reason for bash read-only prefix trust approval
	PlanModeBashTrustDeclined              string // model-facing denial after bash read-only prefix rejection
	SandboxEscapeSubjectFallback           string // fallback subject for a one-shot unconfined sandbox escape approval
	SandboxEscapeSubjectPrefix             string // subject prefix before the shell command for one-shot unconfined escape approval
	SandboxEscapeWrapReason                string // reason when no OS sandbox can wrap the command
	SandboxEscapeRuntimeReason             string // fallback reason when an OS sandbox cannot start the command
	SandboxEscapeDeclined                  string // model-facing denial when the user declines a one-shot unconfined retry
	ApprovalToolLabelConfigWrite           string // user-facing label for patty-managed config write approvals
	ConfigWriteSubjectPrefix               string // subject prefix before the config file path for managed config write approval
	ConfigWriteReason                      string // reason shown for managed config write approval
	ConfigWriteDeclined                    string // model-facing denial when the user declines a managed config write
	ConfigWriteApprovalChoices             string // approval choice list for managed config write prompts
	PermissionSavedFmt                     string // permission rule saved notice: path, rule
	PermissionAlreadyAllowedFmt            string // permission rule already covered notice: path, rule
	PermissionSaveFailedFmt                string // permission rule save failure notice: rule, error
	PlanModeReadOnlyCommandTrustSavedFmt   string // plan-mode bash read-only prefix saved notice: path, prefix
	PlanModeReadOnlyCommandTrustAlreadyFmt string // plan-mode bash read-only prefix already covered notice: path, prefix
	PlanModeReadOnlyCommandTrustFailedFmt  string // plan-mode bash read-only prefix save failure notice: prefix, error
	DiffFoldedFmt                          string // "… +%d more lines" footer when a writer diff is folded
	DiffFoldEnabledFmt                     string // notice when /diff-fold enables folding, %d = line limit
	DiffFoldDisabled                       string // notice when /diff-fold disables folding (shows all lines)

	AskTypeSomething   string // the "type your own answer" option label
	AskTypingHint      string // shown on that row while entering free text
	AskChatInstead     string // the "don't pick, just chat" option label
	ChatStatusQuestion string // shortcuts hint while a question card is open
	StatusResumePicker string // status tag while the resume picker is open (e.g. "select session")
	AskSubmitTitle     string // submit-tab title in the ask tool question card
	AskUnanswered      string // placeholder for an unanswered ask question
	AskSubmitHint      string // submit-tab keyboard hint

	OutputStyleNone              string // no styles available
	OutputStylePickerTitle       string // /output-style picker title
	OutputStyleExplanatory       string // builtin explanatory style label
	OutputStyleExplanatoryDesc   string // builtin explanatory style description
	OutputStyleLearning          string // builtin learning style label
	OutputStyleLearningDesc      string // builtin learning style description
	OutputStyleConcise           string // builtin concise style label
	OutputStyleConciseDesc       string // builtin concise style description
	OutputStyleAlreadyFmt        string // picking the active style again
	OutputStyleSwitchUnavailable string // rebuild unavailable in this session
	OutputStyleSwitchBusy        string // active work blocks the style switch
	OutputStyleSwitchingFmt      string // style switch in progress
	OutputStyleSwitchedFmt       string // style switch succeeded
	ThemeHeader                  string // header above the /theme listing
	ThemeHint                    string // how to select a theme
	ThemeChangedFmt              string // "/theme <name>" succeeded
	ThemeUnknownFmt              string // "/theme <name>" unknown
	LanguageHeader               string // header above the /language listing
	LanguageHint                 string // how to select a language
	LanguageChangedFmt           string // "/language <tag>" succeeded, %s = saved tag, %s = resolved tag
	CurrencyHeader               string // header above the /currency listing
	CurrencyHint                 string // how to select a pricing currency
	CurrencyChangedFmt           string // "/currency <mode>" succeeded, %s = saved mode, %s = resolved currency
	RuntimeRefreshBusy           string // runtime-affecting setting cannot change while work is active
	RuntimeRefreshUnavailable    string // current session cannot rebuild after a runtime-affecting setting change

	CompactionWorking string // shown while the summarizer runs
	CompactionTitle   string // card header before "· N messages · <trigger>"
	CompactionUnit    string // the noun counted, e.g. "messages"
	CompactionAuto    string // trigger label: reached the window threshold
	CompactionManual  string // trigger label: user ran /compact

	ExtFormFieldsHint string // form card: field values are collected through the usual prompts
	ExtRunActionFmt   string // card action hint, one %s = the /<plugin>:<action> slash name

	SlashCompactFailed           string // "/compact" errored, prefixed before the underlying error
	SlashNewDone                 string // "/new" succeeded
	SlashNewFailed               string // "/new" errored
	SlashClearPrompt             string // "/clear" destructive confirmation prompt
	SlashClearDone               string // "/clear" succeeded
	SlashClearFailed             string // "/clear" errored
	SlashClsDone                 string // "/cls" succeeded
	SlashTodoCleared             string // "/todo" dismissed the pinned task list
	SlashUnknown                 string // shown when the user types an unrecognised "/cmd"
	SlashUnknownSentAsMessage    string // suffix: the unrecognised "/cmd" line was sent as a regular message
	SlashPromptEmpty             string // an MCP prompt returned no text to send
	SlashMCPNone                 string // /mcp when no MCP servers are connected
	CtrlCQuitHint                string // shown on first Ctrl+C while idle; second press exits
	CompHintSlash                string // key hint footer under the slash-command menu
	CompHintFile                 string // key hint footer under the @ file/resource menu
	MouseCopiedHint              string // transient status-line hint after a mouse/Ctrl+C selection copy
	ClipboardCopyOSC52Hint       string // copy was sent through OSC 52 because the session is remote
	ClipboardCopyFallbackHint    string // native clipboard failed and copy fell back to OSC 52
	ClipboardTextPasteRemoteHint string // mouse paste cannot read the user's local clipboard/PRIMARY selection over SSH
	ClipboardTextPasteFailedFmt  string // text clipboard read failed, one %v
	ClipboardImagePastingHint    string // shown while an image is being read from the system clipboard
	ClipboardImagePasteFailedFmt string // image clipboard read failed, one %v
	MouseCaptureOnHint           string // "/mouse" turned in-app mouse handling back on
	MouseCaptureOffHint          string // "/mouse" released mouse capture to the terminal
	MouseCaptureTag              string // persistent status-line marker while mouse capture is off

	ShellExecEmpty      string // bare "!" with no command
	ShellExecFailedFmt  string // "shell command failed: %v"
	ShellExecTimeoutFmt string // "shell command timed out (> %s)"
	ShellModeHint       string // status line hint when input starts with !

	CmdNew              string // /new
	CmdClear            string // /clear
	CmdCls              string // /cls
	CmdCompact          string // /compact
	CmdRewind           string // /rewind
	CmdTree             string // /tree
	CmdBranch           string // /branch
	CmdSwitchBranch     string // /switch
	CmdResume           string // /resume
	CmdRename           string // /rename
	CmdModel            string // /model
	CmdStatus           string // /status
	CmdDocs             string // /docs
	CmdMemory           string // /memory
	CmdMigrate          string // /migrate
	CmdGoal             string // /goal
	CmdRemember         string // /remember
	CmdForget           string // /forget
	CmdMcp              string // /mcp
	CmdRemote           string // /remote
	CmdHooks            string // /hooks
	CmdPlugins          string // /plugins
	PluginComingSoon    string // /plugins placeholder notice
	CmdPasteImage       string // /paste-image
	CmdOutputStyle      string // /output-style
	CmdTheme            string // /theme
	CmdLanguage         string // /language
	CmdCurrency         string // /currency
	CmdSkill            string // /skills
	CmdVerbose          string // /verbose
	CmdReloadCmd        string // /reload-cmd
	CmdReload           string // /reload
	CmdDiffFold         string // /diff-fold
	CmdSandbox          string // /sandbox
	CmdEffort           string // /effort
	CmdMouse            string // /mouse
	CmdReasonLang       string // /reasoning-language
	CmdHelp             string // /help
	CmdTodo             string // /todo
	CmdQuit             string // /quit (also accepts /exit as hidden alias)
	CmdCopy             string // /copy
	CmdExport           string // /export
	SlashCopyDone       string // "/copy" succeeded
	SlashCopyEmpty      string // no assistant response to copy
	SlashCopyListHeader string // header shown before the numbered list
	SlashExportDoneFmt  string // "/export" succeeded, %s = file path
	SlashExportEmpty    string // no messages to export
	ArgSkillShow        string // /skills show
	ArgSkillNew         string // /skills new
	ArgSkillPaths       string // /skills paths
	ArgMcpAdd           string // /mcp add
	ArgMcpRemove        string // /mcp remove
	ArgMcpConnected     string // /mcp remove <server> tag
	ArgHooksList        string // /hooks list
	ArgModelCurrent     string // /model <ref> active tag
	ArgEffortAuto       string // /effort auto
	ArgEffortLow        string // /effort low
	ArgEffortMedium     string // /effort medium
	ArgEffortHigh       string // /effort high
	ArgEffortXHigh      string // /effort xhigh
	ArgEffortMax        string // /effort max
	ArgThemeCurrent     string // /theme <style> active tag
	ArgLanguageAuto     string // /language auto
	ArgLanguageEn       string // /language en
	ArgLanguageKo       string // /language ko-KR

	ListModelsHeaderFmt string // "models (active: %s)"
	ListModelsHint      string // how to switch
	ListMemorySaved     string // "saved memories"
	ListMemoryArchived  string // "archived memories"
	ListMemoryNone      string // no memory docs
	ListSkillsHeaderFmt string // "skills (%d)"
	ListSkillsNone      string // no skills
	ListHooksHeaderFmt  string // "hooks (%d active)"
	ListHooksNone       string // no hooks
	ListMcpHeader       string // "mcp servers"
	ListMcpNone         string // no mcp servers

	MemoryEditHint               string
	ForgetUsage                  string
	ForgetDoneFmt                string
	QuickRememberEmpty           string
	QuickRememberDoneFmt         string
	GoalEmpty                    string
	GoalCurrentFmt               string
	GoalSetFmt                   string
	GoalCleared                  string
	GoalNotRunning               string
	GoalNotPaused                string
	GoalPaused                   string
	GoalPausedReason             string
	GoalPausedFmt                string // %s = stop cause
	GoalBudgetExtended           string
	GoalRuntimeFmt               string // turns used/limit, tokens used, no-progress, extensions
	GoalRuntimeLastReason        string
	ModelSwitchUnavailable       string
	ModelSwitchBusy              string
	ModelAlreadyOnFmt            string
	ModelSwitchingFmt            string
	ModelSwitchedFmt             string
	ModelListHeader              string
	QuickPickerModelTitle        string
	QuickPickerProviderTitle     string
	QuickPickerSearchLabel       string
	QuickPickerNoMatches         string
	QuickPickerMoreAboveFmt      string
	QuickPickerMoreBelowFmt      string
	QuickPickerHint              string
	QuickPickerActive            string
	QuickPickerProviderFmt       string
	QuickPickerExtensionFmt      string
	QuickPickerExtensionModel    string
	ViewCommandsHeader           string
	ViewBuiltInSection           string
	ViewCustomSection            string
	ViewSkillsSection            string
	ViewMCPPromptsSection        string
	ViewHelpHint                 string
	ViewModelHint                string
	ClearContextDetails          string
	ConfirmLabel                 string
	CancelLabel                  string
	CopyPickerHint               string
	RuntimeSwitchPending         string
	RuntimeReloadQueued          string // /reload queued behind active work; the idle drain runs it
	RuntimeReloaded              string // /reload completed (no generation available)
	RuntimeReloadedGenerationFmt string // /reload completed; %d is the runtime build generation
	RewindNone                   string
	RewindCodeConversation       string
	RewindConversationOnly       string
	RewindCodeOnly               string
	RewindFork                   string
	RewindSummarizeFrom          string
	RewindSummarizeUpto          string
	RewindPickTitle              string
	RewindPickHint               string
	RewindRestoreTitleFmt        string
	RewindApplyHint              string
	RewindCoverageTitle          string
	RewindCoverageWarningFmt     string
	RewindConfirmHint            string
	RewindUnavailableFmt         string
	RewindEmpty                  string

	SkillPickerAvailableFmt      string
	SkillPickerMatchingFmt       string // "%d matching · %d total" when searching
	SkillPickerHint              string
	SkillPickerDetailHint        string
	SkillPickerSearchEmpty       string
	SkillPickerSearchPlaceholder string
	SkillPickerSourceTitle       string
	SkillPickerSourceActiveFmt   string
	SkillPickerSourceHint        string
	SkillPickerDiagHidden        string
	SkillPickerDiagShown         string
	SkillPickerBuiltinSource     string
	SkillPickerRescanned         string
	SkillPickerNoDescription     string
	SkillPickerScopeProject      string
	SkillPickerScopeCustom       string
	SkillPickerScopeGlobal       string
	SkillPickerScopeBuiltin      string
	SkillPickerSubagent          string
	SkillPickerAvailableLabel    string
	SkillPickerDisabledLabel     string
	SkillPickerNoChanges         string
	SkillPickerSourceSkillsHint  string
	SkillPickerSourceSkillsEmpty string
	SkillPickerActionToggle      string
	SkillPickerActionDelete      string
	SkillPickerDeleteTitleFmt    string // "Delete skill %s?"
	SkillPickerDeleteConfirm     string
	SkillPickerDeleteCancel      string
	SkillPickerDeleteHint        string
	SkillPickerDeletedFmt        string // "deleted skill %s"
	SkillPickerMoreAboveFmt      string // "↑ %d more above"
	SkillPickerMoreBelowFmt      string // "↓ %d more below"
	SkillPickerTokenFmt          string // "~%d tok"
	SkillPickerDetailMetaFmt     string // "Scope: %s  Run as: %s"
	SkillPickerSkillsUnit        string // "skills" (used as "%d skills")
	SkillPickerLinesUnit         string // "lines" (used as "+N more lines")
	SkillPickerStatusLabel       string // shown in the TUI status bar while picker is open
	SkillPickerStatusOK          string // "ok" path status label
	SkillPickerStatusMissing     string // "missing" path status label
	SkillPickerStatusNotDir      string // "not-directory" path status label
	SkillPickerStatusUnreadable  string // "unreadable" path status label

	EnterAPIKeysHeader       string // header before the per-env-var prompts
	WroteFileFmt             string // "Wrote %s" — used for patty.toml and .env both
	SetupComplete            string // success line at end of init
	SetupCancelled           string // shown when the user aborts the wizard
	TryHintFmt               string // "Try: %s" — %s = command to try (styled)
	NextHint                 string // non-interactive post-write hint
	ConfirmReconfigureFmt    string // "%s already exists. Reconfigure and overwrite?"
	NotOverwritingFmt        string // non-interactive overwrite refusal
	SetupManagerTitle        string
	SetupAddOpenAI           string
	SetupAddAnthropic        string
	SetupProviderExistsFmt   string
	SetupSaveExit            string
	SetupSaveExitDesc        string
	SetupCancel              string
	SetupCancelDesc          string
	SetupModelsUnit          string
	SetupKeySet              string
	SetupKeyMissing          string
	SetupDefaultBadge        string
	SetupProviderActionsFmt  string
	SetupEditProvider        string
	SetupUpdateKey           string
	SetupTestRefresh         string
	SetupSetDefault          string
	SetupRemoveProvider      string
	SetupBack                string
	SetupPromptModels        string
	SetupSharedKeyWarningFmt string
	SetupPromptAPIKeyFmt     string
	SetupSelectDefaultModel  string
	SetupConfirmRemoveFmt    string
	SetupSummaryTitle        string
	SetupSummaryAddedFmt     string
	SetupSummaryEditedFmt    string
	SetupSummaryRemovedFmt   string
	SetupSummaryDefaultFmt   string
	SetupSummaryKeysFmt      string
	SetupSummaryNoChanges    string
	SetupConfirmSave         string
	SetupConcurrentChangeFmt string

	FetchingModelsFmt          string // "Fetching models for %s..."
	FetchModelsSuccessFmt      string // "Found %d models for %s"
	FetchModelsFailedFmt       string // "Failed to fetch models for %s: %v"
	FetchModelsUsingPresetsFmt string // "Live fetch unavailable for %s, using preset model list"
	SelectModelsLabel          string // "Select models to enable for %s"
	CustomFetchEmpty           string // "/models returned an empty list — falling back to manual entry"
	AnthropicFetchEmpty        string // "/models returned an empty list — Anthropic-compatible providers usually don't expose one, falling back to manual entry"
	APIKeyAlreadySetFmt        string // "reusing existing value for %s"
	APIKeyResetPromptFmt       string // "Re-enter %s?"
	InvalidAPIKeyEnvFmt        string // "%q is not a valid API Key variable name..."
	RepairedAPIKeyEnvFmt       string // "provider %s: replaced invalid api_key_env %q with %q"

	CustomProviderDesc   string // "Add third-party OpenAI compatible model"
	CustomAddMethodLabel string // "Select add method"
	CustomMethodManual   string // "Enter model name manually"
	CustomMethodURL      string // "Fetch models from URL"
	CustomPromptModel    string // "Enter model name"
	CustomPromptBaseURL  string // "Enter Base URL"
	CustomPromptKeyEnv   string // "Enter API Key env var name"
	CustomPromptAPIKey   string // "Enter API Key"
	CustomAddedFmt       string // "Added custom model: %s"

	AnthropicProviderDesc          string // "Add Anthropic API compatible model"
	AnthropicAddMethodLabel        string // "Select add method"
	AnthropicMethodManual          string // "Enter model name manually"
	AnthropicMethodURL             string // "Fetch models from URL"
	AnthropicPromptModel           string // "Enter model name"
	AnthropicPromptBaseURL         string // "Enter Base URL"
	AnthropicPromptKeyEnv          string // "Enter API Key env var name"
	AnthropicPromptAPIKey          string // "Enter API Key"
	AnthropicAddedFmt              string // "Added Anthropic compatible model: %s"
	AnthropicFetchingModelsFmt     string // "Fetching models for %s..."
	AnthropicFetchModelsSuccessFmt string // "Found %d models for %s"
	AnthropicFetchModelsFailedFmt  string // "Failed to fetch models for %s: %v"
	AnthropicSelectModelsLabel     string // "Select models to enable for %s"

	RemoteConnectingFmt       string // "connecting to %s…"
	RemoteConnectedFmt        string // "connected to %s"
	RemoteReconnectingFmt     string // "reconnecting to %s (attempt %d)…"
	RemoteDegradedFmt         string // "connected to %s but some forwards are down"
	RemoteDisconnected        string // "disconnected"
	RemoteServeReadyFmt       string // "remote serve ready: %s"
	RemoteHostKeyPromptFmt    string // "host %s key (%s): %s"
	RemotePassphrasePromptFmt string // "passphrase for %s:"
	RemotePasswordPromptFmt   string // "password for %s:"
	RemoteBootstrapStepFmt    string // "remote serve: %s %s"
	RemoteNoHostsHint         string // "no remote hosts configured; add one with `patcode remote add`"

	UnknownCommandFmt         string // "unknown command %q"
	UsageRunHint              string // "usage: patcode run [--model NAME] <task>"
	ErrorPrefix               string // "error:" — prefix for fatal-error output
	ReconfigureOnUnknownModel string // shown when the configured model no longer resolves and setup is re-run
	WriteConfigErr            string // "write config:" — prefix for write failure
	WriteEnvErr               string // "write .env:" — prefix for env-write failure

	ProviderErrBadRequest          string // 400
	ProviderErrAuth                string // 401 — no key configured / sent
	ProviderErrAuthRejected        string // 401 — a key was sent but the server rejected it
	ProviderErrInsufficientBalance string // 402
	ProviderErrUnprocessable       string // 422
	ProviderErrInputSensitive      string // MiniMax 1026
	ProviderErrOutputSensitive     string // MiniMax 1027
	ProviderErrRateLimited         string // 429
	ProviderErrServer              string // 500
	ProviderErrServerBusy          string // 503

	SelectOneHint      string // "(↑/↓ · Enter · q to cancel)"
	SelectManyHint     string // "(↑/↓ · Space · Enter · q)"
	SelectMoreAboveFmt string // "↑ %d more above"
	SelectMoreBelowFmt string // "↓ %d more below"
	SelectSearchHint   string // "/ to search · Esc to cancel"

	CmdProvider          string // /provider
	ProviderListHeader   string // header for /provider list
	ProviderAlreadyOnFmt string // already on provider
	ProviderUnknownFmt   string // unknown provider
	ProviderPickLabel    string // label for provider model picker
	ProviderNoModelsFmt  string // provider has no models

	UpgradeChecking            string // "Checking for updates…"
	UpgradeChannelDeprecated   string // legacy channel selection is ignored
	UpgradeDevBuild            string // dev builds cannot self-update
	UpgradeFetchFailed         string // "failed to check for updates: %v"
	UpgradeInvalidVersion      string // remote version not valid semver
	UpgradeAlreadyLatest       string // already on the latest version
	UpgradeForcing             string // "Reinstalling the same version…"
	UpgradeAvailableFmt        string // "Current: %s → Latest: %s"
	UpgradeNoAssetFmt          string // "no binary found for %s"
	UpgradeDownloadingFmt      string // "Downloading %s (%s)…"
	UpgradeDownloadFailed      string // "download failed: %v"
	UpgradeVerifying           string // "Verifying checksum…"
	UpgradeChecksumFailed      string // "could not fetch checksum file: %v"
	UpgradeChecksumMismatchFmt string // SHA256 mismatch detail
	UpgradeChecksumNotFoundFmt string // asset not listed in SHA256SUMS
	UpgradeExtractFailed       string // "failed to extract binary: %v"
	UpgradeApplying            string // "Replacing binary…"
	UpgradeApplyFailed         string // "failed to apply update: %v"
	UpgradeSuccessFmt          string // "Updated %s → %s"

	ReportNoPending           string
	ReportHeaderFmt           string
	ReportCapturedFmt         string
	ReportPreviewOnlyFmt      string
	ReportSendPrompt          string
	ReportKept                string
	ReportDeletedFmt          string
	ReportSentFmt             string
	ReportConfigFailedFmt     string
	ReportUploadFailedFmt     string
	ReportSentDeleteFailedFmt string
	ReportUsageBody           string

	CLITelemetryConsentNotice           string
	CLITelemetryConsentPrompt           string
	CLITelemetryConsentInvalid          string
	CLITelemetryConsentSaveFailedFmt    string
	CLITelemetryConsentCleanupFailedFmt string

	UsageBody string // full multi-line help text
}

func (m Messages) ProviderStatusMessage(status int) string {
	switch status {
	case 400:
		return m.ProviderErrBadRequest
	case 401, 403:
		return m.ProviderErrAuth
	case 402:
		return m.ProviderErrInsufficientBalance
	case 422:
		return m.ProviderErrUnprocessable
	case 429:
		return m.ProviderErrRateLimited
	case 500:
		return m.ProviderErrServer
	case 503:
		return m.ProviderErrServerBusy
	}
	return ""
}

var (
	M               = Korean
	currentLanguage = "ko"
)

func CurrentLanguage() string {
	return currentLanguage
}

func DetectLanguage(override string) string {
	// Korean is the product default regardless of the host OS locale. English
	// is opt-in through an explicit config/runtime override or PATTY_LANG.
	for _, c := range []string{override, os.Getenv("PATTY_LANG")} {
		if tag := normalize(c); tag != "" {
			return setLanguage(tag)
		}
	}
	return setLanguage("ko") // Korean is the default; English is opt-in
}

func setLanguage(tag string) string {
	switch tag {
	case "en":
		currentLanguage = "en"
		M = English
	case "ko":
		currentLanguage = "ko"
		M = Korean
	default:
		currentLanguage = "ko"
		M = Korean
	}
	return currentLanguage
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-") // ko_KR.UTF-8 → ko-kr.utf-8 (POSIX locales use underscores)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "ko") || strings.Contains(s, "korean") || strings.Contains(s, "한국어") {
		return "ko"
	}
	if strings.HasPrefix(s, "en") || strings.Contains(s, "english") {
		return "en"
	}
	return ""
}
