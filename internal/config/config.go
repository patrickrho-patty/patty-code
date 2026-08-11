package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	fileencoding "patty/internal/fileutil/encoding"
	"patty/internal/netclient"
	"patty/internal/provider"
)

var validSkillName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func IsValidSkillName(name string) bool { return validSkillName.MatchString(name) }

func SkillNameKey(name string) string {
	name = strings.TrimSpace(name)
	if !IsValidSkillName(name) {
		return ""
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(name)
	}
	return name
}

type Config struct {
	ConfigVersion    int                 `toml:"config_version"`
	DefaultModel     string              `toml:"default_model"`
	Language         string              `toml:"language"` // ui/model language tag (e.g. "ko-KR"); empty = auto-detect from $LANG / $PATTY_LANG
	CredentialsStore string              `toml:"credentials_store"`
	UI               UIConfig            `toml:"ui"`
	CLI              CLIConfig           `toml:"cli"`
	Desktop          DesktopConfig       `toml:"desktop"`
	Telemetry        TelemetryConfig     `toml:"telemetry"`
	Notifications    NotificationsConfig `toml:"notifications"`
	Agent            AgentConfig         `toml:"agent"`
	Providers        []ProviderEntry     `toml:"providers"`
	Tools            ToolsConfig         `toml:"tools"`
	Permissions      PermissionsConfig   `toml:"permissions"`
	Sandbox          SandboxConfig       `toml:"sandbox"`
	Network          NetworkConfig       `toml:"network"`
	Environment      EnvironmentConfig   `toml:"environment"`
	Plugins          []PluginEntry       `toml:"plugins"`
	Skills           SkillsConfig        `toml:"skills"`
	Statusline       StatuslineConfig    `toml:"statusline"`
	LSP              LSPConfig           `toml:"lsp"`
	Bot              BotConfig           `toml:"bot"`
	Serve            ServeConfig         `toml:"serve"`
	Secrets          SecretsConfig       `toml:"secrets"`
	Remote           RemoteConfig        `toml:"remote"`

	systemPromptFileSource     promptFileSource
	providerSources            map[string]providerSourceScope
	shadowedProjectProviders   []ProviderEntry
	ignoredProjectDefaultModel string
	ignoredLegacyStepLimits    bool
	expansionEnv               map[string]string
	pluginPackageOwners        map[string]string
	pluginPackageSkillOwners   map[string][]string
	pluginPackageAgentOwners   map[string][]string
	editLoadErr                error
	loadWarnings               []string
}

type promptFileSource uint8

const (
	promptFileSourceUnknown promptFileSource = iota
	promptFileSourceUser
	promptFileSourceProject
)

type systemPromptFileError struct {
	configured string
	candidates []string
	errors     []error
	allMissing bool
}

func (e *systemPromptFileError) Error() string {
	detail := "could not be read from any configured location"
	if e.allMissing {
		detail = "not found at any configured location"
	}
	message := fmt.Sprintf("system_prompt_file %q %s: %s", e.configured, detail, strings.Join(e.candidates, ", "))
	if !e.allMissing && len(e.errors) > 0 {
		message += ": " + errors.Join(e.errors...).Error()
	}
	return message
}

func (e *systemPromptFileError) Unwrap() error { return errors.Join(e.errors...) }

func IsMissingSystemPromptFile(err error) bool {
	var target *systemPromptFileError
	return errors.As(err, &target) && target.allMissing
}

type TelemetryConfig struct {
	CLIMetrics string `toml:"cli_metrics"` // auto|on|off; empty means consent has not been requested
}

func (c *Config) CLITelemetryConfigured() bool {
	if c == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(c.Telemetry.CLIMetrics)) {
	case "auto", "on", "off":
		return true
	default:
		return false
	}
}

func (c *Config) CLITelemetryMode() string {
	if c == nil {
		return "auto"
	}
	switch strings.ToLower(strings.TrimSpace(c.Telemetry.CLIMetrics)) {
	case "on":
		return "on"
	case "off":
		return "off"
	default:
		return "auto"
	}
}

func (c *Config) LoadWarnings() []string {
	if c == nil || len(c.loadWarnings) == 0 {
		return nil
	}
	out := make([]string, len(c.loadWarnings))
	copy(out, c.loadWarnings)
	return out
}

func (c *Config) HasLoadWarnings() bool {
	return c != nil && len(c.loadWarnings) > 0
}

func (c *Config) addLoadWarning(msg string) {
	if c == nil {
		return
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	c.loadWarnings = append(c.loadWarnings, msg)
}

func (c *Config) IgnoredLegacyAgentStepLimits() bool {
	return c != nil && c.ignoredLegacyStepLimits
}

func (c *Config) IgnoredProjectDefaultModel() string {
	if c == nil {
		return ""
	}
	return c.ignoredProjectDefaultModel
}

type SecretsConfig struct {
	FilterSubprocessEnv   bool `toml:"filter_subprocess_env"`
	ProtectSensitiveFiles bool `toml:"protect_sensitive_files"`
}

type providerSourceScope string

const (
	providerSourceUser    providerSourceScope = "user"
	providerSourceProject providerSourceScope = "project"
)

type UIConfig struct {
	Theme          string `toml:"theme"`           // auto|dark|light; empty resolves to auto
	ThemeStyle     string `toml:"theme_style"`     // seoul-night|ink-night|hanji-light|jade-night and legacy aliases
	ShortcutLayout string `toml:"shortcut_layout"` // classic|desktop; accepted for compatibility
	CloseBehavior  string `toml:"close_behavior"`  // legacy desktop close behavior; prefer desktop.close_behavior
	ShowReasoning  bool   `toml:"show_reasoning"`  // Ctrl+O / /verbose: show thinking text in CLI; false = collapsed
	ShowTurnUsage  bool   `toml:"show_turn_usage"` // show per-request token/cost receipts in the CLI/TUI transcript
	CursorShape    string `toml:"cursor_shape"`    // block|underline|bar; empty defaults to bar
}

type CLIConfig struct {
	UpdateChannel string `toml:"update_channel"`
}

type DesktopConfig struct {
	Language                string   `toml:"language"`                   // auto|en|ko-KR; empty/auto = browser/OS auto-detect
	Currency                string   `toml:"currency"`                   // user-global auto|CNY|USD pricing preference shared by desktop and CLI
	LayoutStyle             string   `toml:"layout_style"`               // classic|workbench|creation; desktop layout style
	Theme                   string   `toml:"theme"`                      // auto|dark|light; empty resolves to auto
	ThemeStyle              string   `toml:"theme_style"`                // graphite|aurora|slate|carbon|nocturne|amber and legacy aliases
	TerminalTheme           string   `toml:"terminal_theme"`             // auto|dark|light; auto follows the desktop app theme
	ExternalOpener          string   `toml:"external_opener"`            // preferred installed app used by the desktop Open control
	CloseBehavior           string   `toml:"close_behavior"`             // quit|background; desktop window close behavior
	DisplayMode             string   `toml:"display_mode"`               // standard|compact (legacy "minimal" maps to compact); transcript display mode
	StatusBarStyle          string   `toml:"status_bar_style"`           // icon|text; desktop status bar metric labels
	StatusBarItems          []string `toml:"status_bar_items"`           // ordered visible desktop status bar items
	DefaultToolApprovalMode string   `toml:"default_tool_approval_mode"` // ask|auto|yolo; defaults to auto for newly-created desktop sessions
	CheckUpdates            *bool    `toml:"check_updates"`              // startup update checks; nil keeps the default enabled
	UpdateChannel           string   `toml:"update_channel"`
	Telemetry               *bool    `toml:"telemetry"`          // anonymous launch ping plus scrubbed next-launch native crash diagnostics; nil keeps the default enabled
	Metrics                 *bool    `toml:"metrics"`            // aggregate desktop metrics (anonymous signal/bucket counts, including lifecycle health; no content); nil keeps the default enabled
	ProviderAccess          []string `toml:"provider_access"`    // desktop-only list of provider entries shown in Settings > Model > Access
	ExpandThinking          bool     `toml:"expand_thinking"`    // true = show reasoning text expanded by default; false = collapsed
	ConversationWidth       string   `toml:"conversation_width"` // standard|full; max transcript width; empty = standard
}

func (c *Config) DesktopExternalOpener() string {
	if c == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.Desktop.ExternalOpener))
}

type NotificationsConfig struct {
	Enabled         bool `toml:"enabled"`
	TurnDone        bool `toml:"turn_done"`
	ApprovalRequest bool `toml:"approval_request"`
	AskRequest      bool `toml:"ask_request"`
}

func (c *Config) EnvironmentEnabled() bool {
	return c == nil || c.Environment.Enabled == nil || *c.Environment.Enabled
}

func (c *Config) UITheme() string {
	theme := strings.ToLower(strings.TrimSpace(c.UI.Theme))
	switch theme {
	case "dark":
		return "dark"
	case "light":
		return "light"
	}
	if descriptor, ok := ResolveCLIThemeStyle(theme); ok {
		return descriptor.Mode
	}
	return "auto"
}

func (c *Config) UIThemeStyle() string {
	if style := normalizeCLIThemeStyle(c.UI.ThemeStyle); style != "" {
		return style
	}
	return normalizeCLIThemeStyle(c.UI.Theme)
}

// uiThemeStyleSetting validates the configured spelling without resolving a
// legacy alias. Runtime consumers use UIThemeStyle's canonical value; config
// rendering preserves accepted user input so load-render-load remains stable.
func (c *Config) uiThemeStyleSetting() string {
	style := strings.ToLower(strings.TrimSpace(c.UI.ThemeStyle))
	if normalizeCLIThemeStyle(style) == "" {
		return ""
	}
	return style
}

func (c *Config) UIShortcutLayout() string {
	switch strings.ToLower(strings.TrimSpace(c.UI.ShortcutLayout)) {
	case "desktop", "dual", "dual-axis", "dual_axis":
		return "desktop"
	default:
		return "classic"
	}
}

func (c *Config) UICursorShape() string {
	switch strings.ToLower(strings.TrimSpace(c.UI.CursorShape)) {
	case "block":
		return "block"
	case "underline":
		return "underline"
	default:
		return "bar"
	}
}

func normalizeThemeStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "graphite", "aurora", "slate", "carbon", "nocturne", "amber", "ember", "midnight", "sandstone", "porcelain", "linen", "glacier":
		return strings.ToLower(strings.TrimSpace(style))
	default:
		return ""
	}
}

// CLIThemeDescriptor is the shared public catalog used by config parsing,
// runtime palettes, and slash completion. Aliases remain accepted inputs but
// are intentionally omitted from user-facing theme lists.
type CLIThemeDescriptor struct {
	Name        string
	Mode        string
	Description string
	Aliases     []string
}

var cliThemeDescriptors = []CLIThemeDescriptor{
	{Name: "seoul-night", Mode: "dark", Description: "deep navy · jade · warm gold", Aliases: []string{"graphite", "ember", "amber"}},
	{Name: "ink-night", Mode: "dark", Description: "warm ink · slate blue · brass", Aliases: []string{"midnight", "slate", "carbon", "nocturne"}},
	{Name: "hanji-light", Mode: "light", Description: "paper · national red/blue · ochre", Aliases: []string{"sandstone", "porcelain", "linen", "glacier"}},
	{Name: "jade-night", Mode: "dark", Description: "green-black · jade · gold", Aliases: []string{"aurora"}},
}

func CLIThemeStyles() []CLIThemeDescriptor {
	out := make([]CLIThemeDescriptor, len(cliThemeDescriptors))
	for i, descriptor := range cliThemeDescriptors {
		out[i] = descriptor
		out[i].Aliases = append([]string(nil), descriptor.Aliases...)
	}
	return out
}

func ResolveCLIThemeStyle(style string) (CLIThemeDescriptor, bool) {
	style = strings.ToLower(strings.TrimSpace(style))
	for _, descriptor := range cliThemeDescriptors {
		if style == descriptor.Name {
			return descriptor, true
		}
		for _, alias := range descriptor.Aliases {
			if style == alias {
				return descriptor, true
			}
		}
	}
	return CLIThemeDescriptor{}, false
}

func normalizeCLIThemeStyle(style string) string {
	if descriptor, ok := ResolveCLIThemeStyle(style); ok {
		return descriptor.Name
	}
	return ""
}

func normalizeDesktopLayoutStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "classic":
		return "classic"
	case "workbench", "workspace":
		return "workbench"
	case "creation":
		return "creation"
	default:
		return "workbench"
	}
}

func normalizeCloseBehavior(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "quit", "exit":
		return "quit"
	default:
		return "background"
	}
}

func (c *Config) DesktopLanguage() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.Language)) {
	case "en":
		return "en"
	case "ko-kr", "ko":
		return "ko-KR"
	default:
		return ""
	}
}

func (c *Config) DesktopCurrency() string {
	if c == nil {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(c.Desktop.Currency)) {
	case "CNY", "RMB", "CNH":
		return "CNY"
	case "USD":
		return "USD"
	default:
		return ""
	}
}

func (c *Config) DesktopTheme() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.Theme)) {
	case "auto":
		return "auto"
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "auto"
	}
}

func (c *Config) DesktopThemeStyle() string {
	return normalizeThemeStyle(c.Desktop.ThemeStyle)
}

func (c *Config) DesktopTerminalTheme() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.TerminalTheme)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return "auto"
	}
}

func (c *Config) DesktopLayoutStyle() string {
	if strings.EqualFold(strings.TrimSpace(c.Desktop.ThemeStyle), "workbench") && strings.TrimSpace(c.Desktop.LayoutStyle) == "" {
		return "workbench"
	}
	return normalizeDesktopLayoutStyle(c.Desktop.LayoutStyle)
}

func (c *Config) DesktopCloseBehavior() string {
	if strings.TrimSpace(c.Desktop.CloseBehavior) != "" {
		return normalizeCloseBehavior(c.Desktop.CloseBehavior)
	}
	return normalizeCloseBehavior(c.UI.CloseBehavior)
}

func (c *Config) UICloseBehavior() string {
	return c.DesktopCloseBehavior()
}

func (c *Config) DesktopDisplayMode() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.DisplayMode)) {
	case "standard":
		return "standard"
	case "compact", "minimal":
		return "compact"
	default:
		return "standard"
	}
}

func (c *Config) DesktopConversationWidth() string {
	if c != nil && strings.EqualFold(strings.TrimSpace(c.Desktop.ConversationWidth), "full") {
		return "full"
	}
	return "standard"
}

func NormalizeToolApprovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return "auto"
	case "yolo", "full", "full-access", "bypass":
		return "yolo"
	default:
		return "ask"
	}
}

func (c *Config) DesktopDefaultToolApprovalMode() string {
	if c == nil {
		return "ask"
	}
	return NormalizeToolApprovalMode(c.Desktop.DefaultToolApprovalMode)
}

func (c *Config) DesktopStatusBarStyle() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.StatusBarStyle)) {
	case "icon":
		return "icon"
	case "text":
		return "text"
	default:
		return "text"
	}
}

var defaultDesktopStatusBarItems = []string{
	"model",
	"workspace",
	"git_branch",
	"cache",
	"cache_avg",
	"session_tokens",
	"turn_tokens",
	"turn_cost",
	"session_turns",
	"context",
	"compact",
	"cost",
	"balance",
}

var knownDesktopStatusBarItems = desktopStatusBarItemSet(defaultDesktopStatusBarItems)

func desktopStatusBarItemSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func DefaultDesktopStatusBarItems() []string {
	return append([]string(nil), defaultDesktopStatusBarItems...)
}

func (c *Config) DesktopStatusBarItems() []string {
	return normalizeDesktopStatusBarItems(c.Desktop.StatusBarItems)
}

func normalizeDesktopStatusBarItems(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		id := strings.TrimSpace(raw)
		if !knownDesktopStatusBarItems[id] || seen[id] {
			continue
		}
		out = append(out, id)
		seen[id] = true
	}
	if len(out) == 0 {
		return DefaultDesktopStatusBarItems()
	}
	return out
}

func (c *Config) DesktopCheckUpdates() bool {
	if c == nil || c.Desktop.CheckUpdates == nil {
		return true
	}
	return *c.Desktop.CheckUpdates
}

func NormalizeCLIUpdateChannel(_ string) string {
	return "stable"
}

func (c *Config) CLIUpdateChannel() string {
	if c == nil {
		return "stable"
	}
	return NormalizeCLIUpdateChannel(c.CLI.UpdateChannel)
}

func NormalizeDesktopUpdateChannel(_ string) string {
	return "stable"
}

func (c *Config) DesktopUpdateChannel() string {
	if c == nil {
		return "stable"
	}
	return NormalizeDesktopUpdateChannel(c.Desktop.UpdateChannel)
}

func (c *Config) ColdResumePruneEnabled() bool {
	if c == nil || c.Agent.ColdResumePrune == nil {
		return true
	}
	return *c.Agent.ColdResumePrune
}

func (c *Config) ResponseLanguage() string {
	if c == nil {
		return "auto"
	}
	return NormalizeLanguage(c.Language)
}

func NormalizeLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "auto", "detect", "default":
		return "auto"
	case "ko-kr", "ko":
		return "ko-KR"
	case "en", "english":
		return "en"
	default:
		return "auto"
	}
}

func (c *Config) ReasoningLanguage() string {
	if c == nil {
		return "auto"
	}
	return NormalizeReasoningLanguage(c.Agent.ReasoningLanguage)
}

func NormalizeReasoningLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "auto", "follow", "conversation", "detect", "default", "model", "model-default", "model_default", "provider":
		return "auto"
	case "ko-kr", "ko":
		return "ko-KR"
	case "en", "english":
		return "en"
	default:
		return "auto"
	}
}

func (c *Config) DesktopTelemetry() bool {
	if c == nil || c.Desktop.Telemetry == nil {
		return true
	}
	return *c.Desktop.Telemetry
}

func (c *Config) DesktopMetrics() bool {
	if c == nil || c.Desktop.Metrics == nil {
		return true
	}
	return *c.Desktop.Metrics
}

type LSPConfig struct {
	Enabled bool                 `toml:"enabled"`
	Servers map[string]LSPServer `toml:"servers"`
}

type LSPServer struct {
	Command     string            `toml:"command"`
	Args        []string          `toml:"args"`
	Env         map[string]string `toml:"env"`
	LanguageID  string            `toml:"language_id"`
	Extensions  []string          `toml:"extensions"`
	InstallHint string            `toml:"install_hint"`
}

type StatuslineConfig struct {
	Command string `toml:"command"`
}

type BotConfig struct {
	Enabled            bool                      `toml:"enabled"`
	Model              string                    `toml:"model"` // bot 에 사용할 모델 이름, 비워두면 default_model 사용
	ToolApprovalMode   string                    `toml:"tool_approval_mode"`
	MaxSteps           int                       `toml:"max_steps"`
	DebounceMs         int                       `toml:"debounce_ms"` // 메시지 병합 창, 밀리초
	QueueMode          string                    `toml:"queue_mode"`  // steer|followup|collect|interrupt
	QueueCap           int                       `toml:"queue_cap"`
	QueueDrop          string                    `toml:"queue_drop"` // summarize|old|new
	IgnoreSelfMessages bool                      `toml:"ignore_self_messages"`
	SelfUserIDs        BotSelfUserIDs            `toml:"self_user_ids"`
	Control            BotControlConfig          `toml:"control"`
	Pairing            BotPairingConfig          `toml:"pairing"`
	Allowlist          BotAllowlist              `toml:"allowlist"`
	Routes             []BotRouteConfig          `toml:"routes"`
	Connections        []BotConnectionConfig     `toml:"connections"`
	DesktopWatchers    []BotDesktopWatcherConfig `toml:"desktop_watchers"`
}

type BotDesktopWatcherConfig struct {
	Platform     string `toml:"platform"`
	ConnectionID string `toml:"connection_id"`
	Domain       string `toml:"domain"`
	ChatType     string `toml:"chat_type"`
	ChatID       string `toml:"chat_id"`
}

type BotSelfUserIDs struct {
	Desktop []string `toml:"desktop"`
}

type BotControlConfig struct {
	Enabled  bool   `toml:"enabled"`
	Addr     string `toml:"addr"`
	TokenEnv string `toml:"token_env"`
}

type BotRouteConfig struct {
	ConnectionID     string `toml:"connection_id"`
	Platform         string `toml:"platform"`
	ChatType         string `toml:"chat_type"`
	ChatID           string `toml:"chat_id"`
	UserID           string `toml:"user_id"`
	ThreadID         string `toml:"thread_id"`
	Model            string `toml:"model"`
	ToolApprovalMode string `toml:"tool_approval_mode"`
	WorkspaceRoot    string `toml:"workspace_root"`
}

type BotAllowlist struct {
	Enabled   bool     `toml:"enabled"`
	AllowAll  bool     `toml:"allow_all"`
	Users     []string `toml:"users"`
	Approvers []string `toml:"approvers"`
	Admins    []string `toml:"admins"`
	Groups    []string `toml:"groups"`
}

type BotPairingConfig struct {
	Enabled               bool `toml:"enabled"`
	RequestTTLMinutes     int  `toml:"request_ttl_minutes"`
	MaxPendingPerPlatform int  `toml:"max_pending_per_platform"`
}

type BotAccessConfig struct {
	Enabled        bool     `toml:"enabled"`
	AllowAll       bool     `toml:"allow_all"`
	PairingEnabled bool     `toml:"pairing_enabled"`
	Users          []string `toml:"users"`
	Groups         []string `toml:"groups"`
	Approvers      []string `toml:"approvers"`
	Admins         []string `toml:"admins"`
}

type BotConnectionConfig struct {
	ID               string                        `toml:"id"`
	Provider         string                        `toml:"provider"`
	Domain           string                        `toml:"domain"`
	Label            string                        `toml:"label"`
	Enabled          bool                          `toml:"enabled"`
	Status           string                        `toml:"status"` // disconnected|pending|connected|error
	Model            string                        `toml:"model"`
	ToolApprovalMode string                        `toml:"tool_approval_mode"`
	WorkspaceRoot    string                        `toml:"workspace_root"`
	Access           BotAccessConfig               `toml:"access"`
	Credential       BotConnectionCredential       `toml:"credential"`
	SessionMappings  []BotConnectionSessionMapping `toml:"session_mappings"`
	LastError        string                        `toml:"last_error"`
	CreatedAt        string                        `toml:"created_at"`
	UpdatedAt        string                        `toml:"updated_at"`
}

type BotConnectionCredential struct {
	AppID        string `toml:"app_id"`
	AppSecretEnv string `toml:"app_secret_env"`
	AccountID    string `toml:"account_id"`
	TokenEnv     string `toml:"token_env"`
}

type BotConnectionSessionMapping struct {
	RemoteID      string `toml:"remote_id"`
	SessionID     string `toml:"session_id"`
	SessionSource string `toml:"session_source"`
	ChatType      string `toml:"chat_type"`
	UserID        string `toml:"user_id"`
	ThreadID      string `toml:"thread_id"`
	Scope         string `toml:"scope"`
	WorkspaceRoot string `toml:"workspace_root"`
	UpdatedAt     string `toml:"updated_at"`
}

type ServeConfig struct {
	AuthMode     string `toml:"auth_mode"`
	Token        string `toml:"token"`
	PasswordHash string `toml:"password_hash"`
	BehindProxy  bool   `toml:"behind_proxy"`
}

type NetworkConfig struct {
	ProxyMode string             `toml:"proxy_mode"`
	ProxyURL  string             `toml:"proxy_url"`
	NoProxy   string             `toml:"no_proxy"`
	Proxy     NetworkProxyConfig `toml:"proxy"`
}

type NetworkProxyConfig struct {
	Type     string `toml:"type"` // http|https|socks5|socks5h
	Server   string `toml:"server"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

func (c *Config) NetworkProxySpec() netclient.ProxySpec {
	return netclient.ProxySpec{
		Mode:        c.Network.ProxyMode,
		URL:         c.expandVars(c.Network.ProxyURL),
		NoProxy:     c.expandVars(c.Network.NoProxy),
		Type:        c.Network.Proxy.Type,
		Server:      c.expandVars(c.Network.Proxy.Server),
		Port:        c.Network.Proxy.Port,
		Username:    c.expandVars(c.Network.Proxy.Username),
		Password:    c.expandVars(c.Network.Proxy.Password),
		DirectHosts: c.directProxyHosts(),
	}
}

func (c *Config) directProxyHosts() []string {
	if c.NetworkProxyMode() == netclient.ModeCustom {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range c.Providers {
		if !p.NoProxy {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(p.BaseURL))
		if err != nil {
			continue
		}
		if h := u.Hostname(); h != "" && !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

func (c *Config) NetworkProxyMode() string {
	return netclient.NormalizeMode(c.Network.ProxyMode)
}

type SkillsConfig struct {
	Paths          []string `toml:"paths"`
	ExcludedPaths  []string `toml:"excluded_paths"`
	DisabledSkills []string `toml:"disabled_skills"`
	MaxDepth       int      `toml:"max_depth"`
}

func (c *Config) SkillCustomPaths() []string {
	var out []string
	for _, p := range c.Skills.Paths {
		if p = c.expandVars(p); strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) SkillExcludedPaths() []string {
	var out []string
	for _, p := range c.Skills.ExcludedPaths {
		if p = c.expandVars(p); strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) SkillMaxDepth() int {
	const (
		defaultDepth = 3
		maxDepth     = 5
	)
	if c == nil || c.Skills.MaxDepth == 0 {
		return defaultDepth
	}
	if c.Skills.MaxDepth < 1 {
		return 1
	}
	if c.Skills.MaxDepth > maxDepth {
		return maxDepth
	}
	return c.Skills.MaxDepth
}

func (c *Config) DisabledSkillNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range c.Skills.DisabledSkills {
		name = strings.TrimSpace(name)
		if !IsValidSkillName(name) {
			continue
		}
		key := SkillNameKey(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

func (c *Config) IsSkillDisabled(name string) bool {
	key := SkillNameKey(name)
	if key == "" {
		return false
	}
	for _, disabled := range c.DisabledSkillNames() {
		if SkillNameKey(disabled) == key {
			return true
		}
	}
	return false
}

type SandboxConfig struct {
	WorkspaceRoot string   `toml:"workspace_root"`
	AllowWrite    []string `toml:"allow_write"`
	ForbidRead    []string `toml:"forbid_read"`
	Bash          string   `toml:"bash"`
	Network       bool     `toml:"network"`
}

func (c *Config) WriteRoots() []string {
	return c.WriteRootsForRoot(".")
}

func (c *Config) WriteRootsForRoot(fallbackRoot string) []string {
	root := c.expandVars(c.Sandbox.WorkspaceRoot)
	if root == "" {
		root = fallbackRoot
		if root == "" || root == "." {
			if wd, err := os.Getwd(); err == nil {
				root = wd
			} else {
				root = "."
			}
		}
	}
	roots := []string{root}
	for _, d := range c.Sandbox.AllowWrite {
		if d = c.expandVars(d); d != "" {
			roots = append(roots, d)
		}
	}
	return roots
}

func (c *Config) AllowWriteRoots() []string {
	var roots []string
	for _, d := range c.Sandbox.AllowWrite {
		if d = c.expandVars(d); d != "" {
			roots = append(roots, d)
		}
	}
	return roots
}

func (c *Config) ForbidReadRoots() []string {
	return c.ForbidReadRootsForRoot(".")
}

func (c *Config) ForbidReadRootsForRoot(fallbackRoot string) []string {
	root := fallbackRoot
	if root == "" || root == "." {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	roots := make([]string, 0, len(c.Sandbox.ForbidRead))
	for _, d := range c.Sandbox.ForbidRead {
		if d = c.expandVars(d); d != "" {
			if !filepath.IsAbs(d) {
				d = filepath.Join(root, d)
			}
			roots = append(roots, d)
		}
	}
	return roots
}

func (c *Config) BashMode() string {
	return c.BashModeForGOOS(runtimeGOOS)
}

func (c *Config) BashModeForGOOS(goos string) string {
	if goos == "windows" {
		return "off"
	}
	switch strings.TrimSpace(c.Sandbox.Bash) {
	case "enforce":
		return "enforce"
	case "off":
		return "off"
	case "":
		return "enforce"
	default:
		return "enforce"
	}
}

type AgentConfig struct {
	SystemPrompt             string            `toml:"system_prompt"`
	SystemPromptFile         string            `toml:"system_prompt_file"`
	MaxSteps                 int               `toml:"max_steps"`
	PlannerMaxSteps          int               `toml:"planner_max_steps"`
	Temperature              float64           `toml:"temperature"`
	PlannerModel             string            `toml:"planner_model"`
	GuardianModel            string            `toml:"guardian_model"`
	GuardianTemperature      float64           `toml:"guardian_temperature"`
	RecoveryModel            string            `toml:"recovery_model"`
	RecoveryTemperature      float64           `toml:"recovery_temperature"`
	SubagentModel            string            `toml:"subagent_model"`
	SubagentModels           map[string]string `toml:"subagent_models"`
	SubagentEffort           string            `toml:"subagent_effort"`
	SubagentEfforts          map[string]string `toml:"subagent_efforts"`
	MaxSubagentDepth         int               `toml:"max_subagent_depth"`
	MaxSubagentConcurrency   int               `toml:"max_subagent_concurrency"`
	MaxParallelWriters       int               `toml:"max_parallel_writers"`
	OutputStyle              string            `toml:"output_style"`
	AutoPlan                 string            `toml:"auto_plan"`
	ReasoningLanguage        string            `toml:"reasoning_language"`
	AutoPlanClassifier       string            `toml:"auto_plan_classifier"`
	SoftCompactRatio         float64           `toml:"soft_compact_ratio"`
	ToolResultSnipRatio      float64           `toml:"tool_result_snip_ratio"`
	CompactRatio             float64           `toml:"compact_ratio"`
	CompactForceRatio        float64           `toml:"compact_force_ratio"`
	Keep                     []string          `toml:"keep"`
	RecentKeep               int               `toml:"recent_keep"`
	ColdResumePrune          *bool             `toml:"cold_resume_prune"`
	PlanModeReadOnlyCommands []string          `toml:"plan_mode_read_only_commands"`
}

type ProviderEntry struct {
	Name              string            `toml:"name"`
	Kind              string            `toml:"kind"`
	BaseURL           string            `toml:"base_url"`
	ChatURL           string            `toml:"chat_url"`
	Model             string            `toml:"model"`      // a single model (back-compat)
	Models            []string          `toml:"models"`     // a vendor's model list (one base_url/key, many models)
	ModelsURL         string            `toml:"models_url"` // auto-fetch models from this URL on startup
	Default           string            `toml:"default"`    // default model when Models is set (else Models[0])
	APIKeyEnv         string            `toml:"api_key_env"`
	PresetID          string            `toml:"preset_id"`      // curated preset identity; UI-only metadata, not sent to model providers.
	PresetVersion     int               `toml:"preset_version"` // curated preset schema version for future migrations.
	Headers           map[string]string `toml:"headers"`        // optional extra HTTP headers for compatible gateways; secrets should stay in api_key_env.
	ExtraBody         map[string]any    `toml:"extra_body"`     // optional extra top-level JSON request body fields for OpenAI-compatible gateways.
	AuthHeader        bool              `toml:"auth_header"`    // for Anthropic-compatible gateways that expect Authorization: Bearer instead of x-api-key.
	ResponsesMode     string            `toml:"responses_mode"`
	ResponsesStateful *bool             `toml:"responses_stateful"`
	resolvedAPIKey    string
	resolvedSource    CredentialSource
	BalanceURL        string                       `toml:"balance_url"` // optional; a provider-specific wallet-balance endpoint (DeepSeek: https://api.deepseek.com/user/balance). Empty = no balance readout.
	ContextWindow     int                          `toml:"context_window"`
	MaxOutputTokens   int                          `toml:"max_output_tokens"`
	Price             *provider.Pricing            `toml:"price"`  // legacy/provider-wide fallback
	Prices            map[string]*provider.Pricing `toml:"prices"` // optional per-model prices; keys are model ids

	persistedOfficialCurrency string

	Thinking          string                           `toml:"thinking"`
	Effort            string                           `toml:"effort"`
	Vision            bool                             `toml:"vision"`
	VisionModels      []string                         `toml:"vision_models"`
	VisionDetail      string                           `toml:"vision_detail"`
	WebSearch         *bool                            `toml:"web_search"`
	ReasoningProtocol string                           `toml:"reasoning_protocol"`
	SupportedEfforts  []string                         `toml:"supported_efforts"`
	DefaultEffort     string                           `toml:"default_effort"`
	ModelOverrides    map[string]ProviderModelOverride `toml:"model_overrides"`
	visionOverride    *bool
	NoProxy           bool `toml:"no_proxy"`
	CacheTTLMinutes   int  `toml:"cache_ttl_minutes"`
}

type ProviderModelOverride struct {
	ReasoningProtocol string   `toml:"reasoning_protocol"`
	SupportedEfforts  []string `toml:"supported_efforts"`
	DefaultEffort     string   `toml:"default_effort"`
	Vision            *bool    `toml:"vision"`
	ContextWindow     int      `toml:"context_window"`
	MaxOutputTokens   int      `toml:"max_output_tokens"`
}

func (e *ProviderEntry) ModelList() []string {
	if len(e.Models) > 0 {
		return e.Models
	}
	if e.Model != "" {
		return []string{e.Model}
	}
	return nil
}

func IsLikelyChatModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	lower := strings.ToLower(model)

	var compoundNonChat = []string{
		"text-embedding", "text-to-speech", "speech-to-text",
	}
	for _, c := range compoundNonChat {
		if strings.Contains(lower, c) {
			return false
		}
	}

	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == ':'
	})
	var nonChatTokens = map[string]bool{
		"asr": true, "stt": true, "tts": true,
		"whisper": true, "embedding": true,
		"moderation": true, "rerank": true, "dall": true,
		"transcription": true,
	}
	for _, tok := range tokens {
		if nonChatTokens[tok] {
			return false
		}
	}
	return true
}

func (e *ProviderEntry) ChatModelList() []string {
	raw := e.ModelList()
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, m := range raw {
		if IsLikelyChatModel(m) {
			out = append(out, m)
		}
	}
	return out
}

func (e *ProviderEntry) DefaultModel() string {
	if e.Default != "" {
		return e.Default
	}
	if l := e.ModelList(); len(l) > 0 {
		return l[0]
	}
	return ""
}

func (e *ProviderEntry) HasModel(m string) bool {
	return slices.Contains(e.ModelList(), m)
}

func (e *ProviderEntry) PriceForModel(model string) *provider.Pricing {
	if e == nil {
		return nil
	}
	if e.Prices != nil {
		if p := e.Prices[strings.TrimSpace(model)]; p != nil {
			return clonePricing(p)
		}
	}
	return clonePricing(e.Price)
}

func (e *ProviderEntry) applyModelPrice() {
	if e == nil {
		return
	}
	e.Price = e.PriceForModel(e.Model)
}

func (e *ProviderEntry) applyModelOverride() {
	if e == nil || len(e.ModelOverrides) == 0 {
		return
	}
	ov, ok := e.modelOverrideForModel(e.Model)
	if !ok {
		return
	}
	if ov.ReasoningProtocol != "" {
		e.ReasoningProtocol = ov.ReasoningProtocol
	}
	if ov.SupportedEfforts != nil {
		e.SupportedEfforts = append([]string(nil), ov.SupportedEfforts...)
	}
	if ov.DefaultEffort != "" || ov.SupportedEfforts != nil {
		e.DefaultEffort = ov.DefaultEffort
	}
	if ov.Vision != nil {
		e.visionOverride = ov.Vision
	}
	if ov.ContextWindow > 0 {
		e.ContextWindow = ov.ContextWindow
	}
	if ov.MaxOutputTokens != 0 {
		e.MaxOutputTokens = ov.MaxOutputTokens
	}
}

func (e *ProviderEntry) modelOverrideForModel(model string) (ProviderModelOverride, bool) {
	model = strings.TrimSpace(model)
	if e == nil || model == "" || len(e.ModelOverrides) == 0 {
		return ProviderModelOverride{}, false
	}
	if ov, ok := e.ModelOverrides[model]; ok {
		return ov, true
	}
	for k, ov := range e.ModelOverrides {
		if strings.EqualFold(strings.TrimSpace(k), model) {
			return ov, true
		}
	}
	return ProviderModelOverride{}, false
}

func clonePricing(p *provider.Pricing) *provider.Pricing {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

type ToolsConfig struct {
	Enabled                  []string             `toml:"enabled"`
	BashTimeoutSeconds       *int                 `toml:"bash_timeout_seconds"`
	MCPStartupTimeoutSeconds *int                 `toml:"mcp_startup_timeout_seconds"`
	MCPCallTimeoutSeconds    *int                 `toml:"mcp_call_timeout_seconds"`
	BackgroundJobs           BackgroundJobsConfig `toml:"background_jobs"`
	Search                   SearchConfig         `toml:"search"`
	Shell                    ShellConfig          `toml:"shell"`
}

const (
	defaultBashTimeoutSeconds             = 120
	defaultMCPStartupTimeoutSeconds       = 30
	defaultMCPCallTimeoutSeconds          = 300
	defaultBackgroundJobStalledWarningSec = 900
	maxBackgroundJobStalledWarningSec     = 86400
)

func (c *Config) BashTimeoutSeconds() int {
	if c.Tools.BashTimeoutSeconds == nil || *c.Tools.BashTimeoutSeconds < 0 {
		return defaultBashTimeoutSeconds
	}
	return *c.Tools.BashTimeoutSeconds
}

func (c *Config) MCPCallTimeoutSeconds() int {
	if c.Tools.MCPCallTimeoutSeconds == nil || *c.Tools.MCPCallTimeoutSeconds <= 0 {
		return defaultMCPCallTimeoutSeconds
	}
	return *c.Tools.MCPCallTimeoutSeconds
}

func (c *Config) MCPStartupTimeoutSeconds() int {
	if c.Tools.MCPStartupTimeoutSeconds == nil || *c.Tools.MCPStartupTimeoutSeconds <= 0 {
		return defaultMCPStartupTimeoutSeconds
	}
	return *c.Tools.MCPStartupTimeoutSeconds
}

type BackgroundJobsConfig struct {
	StalledWarningSeconds *int `toml:"stalled_warning_seconds"`
}

func (c *Config) BackgroundJobStalledWarningSeconds() int {
	if c.Tools.BackgroundJobs.StalledWarningSeconds == nil || *c.Tools.BackgroundJobs.StalledWarningSeconds < 0 {
		return defaultBackgroundJobStalledWarningSec
	}
	if *c.Tools.BackgroundJobs.StalledWarningSeconds > maxBackgroundJobStalledWarningSec {
		return maxBackgroundJobStalledWarningSec
	}
	return *c.Tools.BackgroundJobs.StalledWarningSeconds
}

type SearchConfig struct {
	Engine string `toml:"engine"`
	RgPath string `toml:"rg_path"`
}

type ShellConfig struct {
	Prefer string `toml:"prefer"`
	Path   string `toml:"path"`
}

type PermissionsConfig struct {
	Mode             string   `toml:"mode"`
	Allow            []string `toml:"allow"`
	Ask              []string `toml:"ask"`
	Deny             []string `toml:"deny"`
	AllowDynamicBash bool     `toml:"allow_dynamic_bash"`
}

type MCPConfigSource string

const (
	MCPSourceUnknown        MCPConfigSource = ""
	MCPSourceUserConfig     MCPConfigSource = "user_config"
	MCPSourceProjectConfig  MCPConfigSource = "project_config"
	MCPSourceProjectMCPJSON MCPConfigSource = "project_mcp_json"
	MCPSourceLegacyUser     MCPConfigSource = "legacy_user_config"
	MCPSourcePluginPackage  MCPConfigSource = "plugin_package"
)

func (s MCPConfigSource) UserAuthorized() bool {
	switch s {
	case MCPSourceUserConfig, MCPSourceLegacyUser, MCPSourcePluginPackage,
		MCPSourceProjectConfig, MCPSourceProjectMCPJSON:
		return true
	default:
		return false
	}
}

func (s MCPConfigSource) ProjectScoped() bool {
	return s == MCPSourceProjectConfig || s == MCPSourceProjectMCPJSON
}

type PluginEntry struct {
	Name                  string            `toml:"name"`
	Type                  string            `toml:"type"` // "stdio" (default) | "http" | "sse"
	Command               string            `toml:"command"`
	Args                  []string          `toml:"args"`
	Env                   map[string]string `toml:"env"`
	URL                   string            `toml:"url"`
	Headers               map[string]string `toml:"headers"`
	StartupTimeoutSeconds int               `toml:"startup_timeout_seconds"`
	CallTimeoutSeconds    int               `toml:"call_timeout_seconds"`
	ToolTimeoutSeconds    map[string]int    `toml:"tool_timeout_seconds"`
	AutoStart             *bool             `toml:"auto_start"`
	Tier                  string            `toml:"tier"`
	Source                MCPConfigSource   `toml:"-" json:"-"`
	expansionEnv          map[string]string
}

func (e PluginEntry) ShouldAutoStart() bool {
	return e.AutoStart == nil || *e.AutoStart
}

func (e PluginEntry) ResolvedTier() string {
	return resolvedMCPTier(e.Tier)
}

func resolvedMCPTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "eager":
		return "eager"
	case "background", "lazy":
		return "background"
	case "":
		return "background"
	default:
		return "background"
	}
}

func (c *Config) AutoStartPlugins() []PluginEntry {
	return c.EnabledPlugins("", DefaultMCPActivationStore())
}

func (c *Config) EnabledPlugins(workspace string, activation *MCPActivationStore) []PluginEntry {
	if c == nil {
		return nil
	}
	out := make([]PluginEntry, 0, len(c.Plugins))
	for _, p := range c.Plugins {
		enabled := p.ShouldAutoStart()
		if activation != nil {
			if resolved, err := activation.IsEnabled(p, workspace); err == nil {
				enabled = resolved
			}
		}
		if enabled {
			out = append(out, p)
		}
	}
	return out
}

const DefaultSystemPrompt = `You are Patty Code, a coding agent.
Use the available tools when they help you complete the user's request.
Keep changes focused and responses concise.`

const UserDecisionPolicy = `User-owned choices: when a consequential decision has no safe, obvious default, call the ask tool so the user can choose. Otherwise proceed with a sensible reversible default. Do not ask in prose when ask is available. In non-interactive runs, state the assumption and take the safest reversible path.`

const LanguagePolicy = `Reply in the same language the user is using in their most recent message: ` +
	`if they write in Korean answer in Korean, in English answer in English, and switch ` +
	`whenever they switch. Let this also guide the language you think in. Always keep code, ` +
	`identifiers, file paths, shell commands, and technical terms in their original form — never translate them.`

func Default() *Config {
	return &Config{
		ConfigVersion:    5,
		DefaultModel:     "patty/medium",
		CredentialsStore: CredentialsStoreAuto,
		UI:               UIConfig{Theme: "auto", ShowTurnUsage: true},
		Desktop:          DesktopConfig{DefaultToolApprovalMode: "auto", ConversationWidth: "standard"},
		Notifications: NotificationsConfig{
			Enabled:         false,
			TurnDone:        true,
			ApprovalRequest: true,
			AskRequest:      true,
		},
		Agent: AgentConfig{
			SystemPrompt:           DefaultSystemPrompt,
			MaxSteps:               0,
			PlannerMaxSteps:        0,
			AutoPlan:               "off",
			SoftCompactRatio:       0.5,
			ToolResultSnipRatio:    0.6,
			CompactRatio:           float64(238123) / float64(248124),
			CompactForceRatio:      0.98,
			MaxSubagentDepth:       2,
			MaxSubagentConcurrency: 6,
			MaxParallelWriters:     3,
		},
		Permissions: PermissionsConfig{Mode: "ask"},
		Sandbox:     SandboxConfig{Network: true},
		LSP:         LSPConfig{Enabled: true},
		Network:     NetworkConfig{ProxyMode: netclient.ModeAuto},
		Bot: BotConfig{
			ToolApprovalMode:   "ask",
			MaxSteps:           25,
			DebounceMs:         1500,
			QueueMode:          "steer",
			QueueCap:           20,
			QueueDrop:          "summarize",
			IgnoreSelfMessages: true,
			Control:            BotControlConfig{Addr: "127.0.0.1:37913", TokenEnv: "PATTY_BOT_CONTROL_TOKEN"},
			Pairing:            BotPairingConfig{Enabled: true, RequestTTLMinutes: 60, MaxPendingPerPlatform: 3},
			Allowlist:          BotAllowlist{Enabled: true},
		},
		Providers: []ProviderEntry{{
			Name:          "patty",
			Kind:          "openai",
			BaseURL:       "https://omni.agents.patty.io/v1",
			Model:         "medium",
			APIKeyEnv:     "AGENTS_PATTY_API_KEY",
			ContextWindow: 248124,
		}},
	}
}

func (c *Config) WriteFile(path string) error {
	return atomicWriteToConfigFile(path, RenderTOMLForScope(c, renderScopeForPath(path)), configFilePerm(path))
}

func (c *Config) Provider(name string) (*ProviderEntry, bool) {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i], true
		}
	}
	return nil, false
}

func (c *Config) ResolveModel(ref string) (*ProviderEntry, bool) {
	if ref == "" {
		return nil, false
	}
	if access := desktopProviderAccessMap(c.Desktop.ProviderAccess); len(access) > 0 {
		ref = retargetDesktopOfficialRef(ref, access)
	}
	if prov, model, ok := strings.Cut(ref, "/"); ok {
		if e, found := c.Provider(prov); found && e.HasModel(model) {
			cp := *e
			cp.Model = model
			cp.applyModelPrice()
			cp.applyModelOverride()
			return &cp, true
		}
	}
	if e, found := c.Provider(ref); found {
		cp := *e
		cp.Model = e.DefaultModel()
		cp.applyModelPrice()
		cp.applyModelOverride()
		return &cp, true
	}
	for i := range c.Providers {
		if c.Providers[i].HasModel(ref) {
			cp := c.Providers[i]
			cp.Model = ref
			cp.applyModelPrice()
			cp.applyModelOverride()
			return &cp, true
		}
	}
	return nil, false
}

func (c *Config) ResolveModelWithFallback(ref string) (resolvedRef string, fallback bool, ok bool) {
	ref = strings.TrimSpace(ref)
	if ref != "" {
		if e, found := c.ResolveModel(ref); found {
			return e.Name + "/" + e.Model, false, true
		}
	}
	if ref != c.DefaultModel && c.DefaultModel != "" {
		if e, found := c.ResolveModel(c.DefaultModel); found && e.Configured() {
			return e.Name + "/" + e.Model, true, true
		}
	}
	for i := range c.Providers {
		p := &c.Providers[i]
		if len(p.ModelList()) == 0 || !p.Configured() {
			continue
		}
		return p.Name + "/" + p.DefaultModel(), true, true
	}
	return "", false, false
}

func (c *Config) ResolveNewSessionChatModel() (resolvedRef string, fallback bool, ok bool) {
	return c.resolveNewSessionChatModel(nil, true)
}

func (c *Config) resolveNewSessionChatModel(providerAllowed func(string) bool, preserveUnknownDefault bool) (resolvedRef string, fallback bool, ok bool) {
	if c == nil {
		return "", false, false
	}
	if providerAllowed == nil {
		providerAllowed = func(string) bool { return true }
	}

	def := strings.TrimSpace(c.DefaultModel)
	keylessDefault := ""
	if def != "" {
		if entry, found := c.ResolveModel(def); found {
			if providerAllowed(entry.Name) && IsLikelyChatModel(entry.Model) {
				if entry.Configured() {
					return def, false, true
				}
				keylessDefault = def
			}
		} else if preserveUnknownDefault {
			return def, false, true
		}
	}

	keylessFallback := ""
	for i := range c.Providers {
		p := &c.Providers[i]
		if !providerAllowed(p.Name) {
			continue
		}
		chatModels := p.ChatModelList()
		if len(chatModels) == 0 {
			continue
		}
		model := chatModels[0]
		for _, candidate := range chatModels {
			if candidate == p.DefaultModel() {
				model = candidate
				break
			}
		}
		resolved := p.Name + "/" + model
		if p.Configured() {
			return resolved, true, true
		}
		if keylessFallback == "" {
			keylessFallback = resolved
		}
	}
	if keylessDefault != "" {
		return keylessDefault, false, true
	}
	if keylessFallback != "" {
		return keylessFallback, true, true
	}
	return "", false, false
}

func (c *Config) ResolveDesktopNewSessionModel() (resolvedRef string, fallback bool, ok bool) {
	if c == nil {
		return "", false, false
	}
	access := desktopProviderAccessMap(c.Desktop.ProviderAccess)
	return c.resolveNewSessionChatModel(func(name string) bool {
		return c.Desktop.ProviderAccess == nil || access[strings.TrimSpace(name)]
	}, false)
}

func (e *ProviderEntry) APIKey() string {
	if e == nil {
		return ""
	}
	if e.resolvedAPIKey != "" {
		return e.resolvedAPIKey
	}
	if e.APIKeyEnv == "" {
		return ""
	}
	value, _, ok := storedCredentialValue(e.APIKeyEnv)
	if !ok {
		return ""
	}
	return value
}

func (e *ProviderEntry) ResolveAPIKeyFromProcessEnvForProbe() {
	if e == nil {
		return
	}
	key := strings.TrimSpace(e.APIKeyEnv)
	if key == "" {
		return
	}
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}
	e.resolvedAPIKey = value
	e.resolvedSource = CredentialSource{Kind: CredentialSourceEnvironment, Label: "setup prompt"}
}

func (e *ProviderEntry) APIKeySourceLabel() string {
	if e == nil || strings.TrimSpace(e.APIKeyEnv) == "" {
		return ""
	}
	if e.resolvedAPIKey != "" {
		return credentialSourceLabel(e.resolvedSource)
	}
	return ResolveCredentialForRootGlobalFirst(".", e.APIKeyEnv).Source.Label
}

func (e *ProviderEntry) RequiresAPIKey() bool {
	if e == nil {
		return false
	}
	if strings.TrimSpace(e.APIKeyEnv) == "" {
		return providerBaseURLRequiresAPIKey(e.BaseURL)
	}
	return !providerBaseURLAllowsMissingAPIKey(e.BaseURL)
}

func providerBaseURLRequiresAPIKey(raw string) bool {
	switch officialProviderHost(raw) {
	case "api.deepseek.com", "api.xiaomimimo.com", "api.openai.com":
		return true
	default:
		return false
	}
}

func providerBaseURLAllowsMissingAPIKey(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.Trim(strings.ToLower(u.Hostname()), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}

func (e *ProviderEntry) Configured() bool {
	return e != nil && (!e.RequiresAPIKey() || e.APIKey() != "")
}

func (c *Config) ResolveSystemPrompt() (string, error) {
	return c.ResolveSystemPromptForRoot(".")
}

func (c *Config) ResolveSystemPromptForRoot(root string) (string, error) {
	path := c.Agent.SystemPromptFile
	if path == "" {
		return c.InlineSystemPrompt(), nil
	}

	if c.systemPromptFileSource == promptFileSourceProject {
		if filepath.IsAbs(path) || !filepath.IsLocal(filepath.Clean(path)) {
			return "", fmt.Errorf("project system_prompt_file %q must be a relative path within the workspace", path)
		}
		candidate := filepath.Join(resolveRoot(root), path)
		b, err := readProjectSystemPromptFile(root, path)
		if err != nil {
			return "", newSystemPromptFileError(path, []string{candidate}, []error{err})
		}
		return strings.TrimSpace(string(b)), nil
	}

	if filepath.IsAbs(path) {
		b, err := fileencoding.ReadFileUTF8(path)
		if err != nil {
			return "", newSystemPromptFileError(path, []string{path}, []error{err})
		}
		return strings.TrimSpace(string(b)), nil
	}

	candidates := []string{filepath.Join(resolveRoot(root), path)}
	if home := PattyHomeDir(); home != "" {
		homeCandidate := filepath.Join(home, path)
		if filepath.Clean(homeCandidate) != filepath.Clean(candidates[0]) {
			candidates = append(candidates, homeCandidate)
		}
	}
	readErrors := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		b, err := fileencoding.ReadFileUTF8(candidate)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
		readErrors = append(readErrors, fmt.Errorf("%s: %w", candidate, err))
	}
	return "", newSystemPromptFileError(path, candidates, readErrors)
}

func readProjectSystemPromptFile(root, path string) ([]byte, error) {
	workspace, err := filepath.Abs(resolveRoot(root))
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	rootHandle, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, fmt.Errorf("open workspace root %q: %w", workspace, err)
	}
	defer rootHandle.Close()
	f, err := rootHandle.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return fileencoding.DecodeToUTF8(b), nil
}

func newSystemPromptFileError(configured string, candidates []string, readErrors []error) error {
	allMissing := len(readErrors) > 0
	for _, err := range readErrors {
		if !errors.Is(err, fs.ErrNotExist) {
			allMissing = false
			break
		}
	}
	return &systemPromptFileError{
		configured: configured,
		candidates: append([]string(nil), candidates...),
		errors:     append([]error(nil), readErrors...),
		allMissing: allMissing,
	}
}

func (c *Config) InlineSystemPrompt() string {
	if strings.TrimSpace(c.Agent.SystemPrompt) == "" {
		return DefaultSystemPrompt
	}
	return c.Agent.SystemPrompt
}

func (c *Config) Validate(model string) error {
	e, ok := c.ResolveModel(model)
	if !ok {
		return fmt.Errorf("unknown model %q (configured: %s)", model, c.providerNames())
	}
	if e.Kind == "" {
		return fmt.Errorf("provider %q: kind is required", model)
	}
	if e.BaseURL == "" {
		return fmt.Errorf("provider %q: base_url is required", model)
	}
	if strings.TrimSpace(e.APIKeyEnv) != "" && !IsValidCredentialKey(e.APIKeyEnv) {
		return fmt.Errorf("provider %q: api_key_env %q is invalid; use letters, numbers, and underscores, not a model name", model, e.APIKeyEnv)
	}
	if e.RequiresAPIKey() && e.APIKey() == "" {
		return fmt.Errorf("provider %q: missing env %s", model, e.APIKeyEnv)
	}
	return nil
}

func (c *Config) providerNames() string {
	names := make([]string, len(c.Providers))
	for i, p := range c.Providers {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
