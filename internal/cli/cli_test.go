package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"patty/internal/agent"
	"patty/internal/boot"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
	"patty/internal/notify"
	"patty/internal/provider"
	"patty/internal/telemetry"
)

func TestChdirTo(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if rc := chdirTo(""); rc != 0 {
		t.Fatalf(`chdirTo("") = %d, want 0`, rc)
	}
	if cwd, _ := os.Getwd(); cwd != orig {
		t.Fatalf(`chdirTo("") moved cwd to %q`, cwd)
	}

	tmp := t.TempDir()
	// Restore CWD before TempDir's RemoveAll runs (LIFO ordering): Windows can't
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if rc := chdirTo(tmp); rc != 0 {
		t.Fatalf("chdirTo(tmp) = %d, want 0", rc)
	}
	got, _ := filepath.EvalSymlinks(mustGetwd(t))
	want, _ := filepath.EvalSymlinks(tmp)
	if got != want {
		t.Fatalf("cwd = %q, want %q", got, want)
	}

	if rc := chdirTo(filepath.Join(tmp, "does-not-exist")); rc != 2 {
		t.Fatalf("chdirTo(missing) = %d, want 2", rc)
	}
}

func TestModelForResumePathUsesStoredModelWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	session := agent.NewSession("sys")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	if err := session.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := agent.SetBranchModelPreserveUpdated(path, "saved/model"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DefaultModel: "default/model",
		Providers: []config.ProviderEntry{
			{Name: "default", Kind: "openai", BaseURL: "https://default.invalid/v1", Model: "model"},
			{Name: "saved", Kind: "openai", BaseURL: "https://saved.invalid/v1", Model: "model"},
		},
	}

	if got := modelForResumePath("", path, cfg); got != "saved/model" {
		t.Fatalf("modelForResumePath = %q, want saved/model", got)
	}
	if got := modelForResumePath("explicit/model", path, cfg); got != "explicit/model" {
		t.Fatalf("explicit model was overwritten: %q", got)
	}
	if got := modelForResumePath("", filepath.Join(dir, "missing.jsonl"), cfg); got != "" {
		t.Fatalf("missing session model = %q, want empty fallback", got)
	}
	cfg.Providers = cfg.Providers[:1]
	if got := modelForResumePath("", path, cfg); got != "" {
		t.Fatalf("unknown stored model = %q, want empty fallback", got)
	}
}

func TestLoadResumableSessionRejectsCleanupPending(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending.jsonl")
	saveTestSession(t, path, "pending prompt")
	if err := agent.MarkCleanupPending(path, "delete"); err != nil {
		t.Fatal(err)
	}

	if _, err := loadResumableSession(path); err == nil || !strings.Contains(err.Error(), "pending cleanup") {
		t.Fatalf("loadResumableSession cleanup-pending error = %v, want pending cleanup", err)
	}
}

func TestRunResumeRejectsCleanupPending(t *testing.T) {
	isolateCLIConfigHome(t)

	path := filepath.Join(t.TempDir(), "pending-run.jsonl")
	saveTestSession(t, path, "pending prompt")
	if err := agent.MarkCleanupPending(path, "delete"); err != nil {
		t.Fatal(err)
	}

	errOut := captureStderr(t, func() {
		if rc := runAgent([]string{"--resume", path, "continue task"}, "dev"); rc != 1 {
			t.Fatalf("run --resume cleanup-pending rc = %d, want 1", rc)
		}
	})
	if !strings.Contains(errOut, "pending cleanup") {
		t.Fatalf("run --resume cleanup-pending stderr = %q, want pending cleanup", errOut)
	}
}

func TestServeResumeRejectsCleanupPending(t *testing.T) {
	isolateCLIConfigHome(t)

	path := filepath.Join(t.TempDir(), "pending-serve.jsonl")
	saveTestSession(t, path, "pending prompt")
	if err := agent.MarkCleanupPending(path, "delete"); err != nil {
		t.Fatal(err)
	}

	errOut := captureStderr(t, func() {
		if rc := runServe([]string{"--resume", path, "--addr", "127.0.0.1:0"}); rc != 1 {
			t.Fatalf("serve --resume cleanup-pending rc = %d, want 1", rc)
		}
	})
	if !strings.Contains(errOut, "pending cleanup") {
		t.Fatalf("serve --resume cleanup-pending stderr = %q, want pending cleanup", errOut)
	}
}

func TestServeRejectsUnknownAuthMode(t *testing.T) {
	isolateCLIConfigHome(t)

	errOut := captureStderr(t, func() {
		if rc := runServe([]string{"--auth", "tokne", "--addr", "127.0.0.1:0"}); rc != 1 {
			t.Fatalf("serve --auth tokne rc = %d, want 1", rc)
		}
	})
	if !strings.Contains(errOut, "auth mode must be none, token, or password") {
		t.Fatalf("serve --auth tokne stderr = %q, want auth mode validation", errOut)
	}
}

func TestServePasswordAuthRequiresPasswordMaterial(t *testing.T) {
	isolateCLIConfigHome(t)

	errOut := captureStderr(t, func() {
		if rc := runServe([]string{"--auth", "password", "--addr", "127.0.0.1:0"}); rc != 1 {
			t.Fatalf("serve --auth password without password rc = %d, want 1", rc)
		}
	})
	if !strings.Contains(errOut, "auth mode password requires --password or serve.password_hash") {
		t.Fatalf("serve --auth password stderr = %q, want password material validation", errOut)
	}
}

func TestReserveNativeScrollbackFrameWritesOnlyNewlines(t *testing.T) {
	var b bytes.Buffer
	reserveNativeScrollbackFrame(&b, 3)
	if got := b.String(); got != "\n\n\n" {
		t.Fatalf("reserveNativeScrollbackFrame wrote %q, want only three newlines", got)
	}

	reserveNativeScrollbackFrame(&b, 0)
	if got := b.String(); got != "\n\n\n" {
		t.Fatalf("reserveNativeScrollbackFrame(0) changed output to %q", got)
	}
}

func TestPrepareNativeScrollbackClearsBeforeFrame(t *testing.T) {
	var b bytes.Buffer
	prepareNativeScrollback(&b, 2)
	if got, want := b.String(), "\x1B[3J\x1B[2J\x1B[H\n\n"; got != want {
		t.Fatalf("prepareNativeScrollback wrote %q, want %q", got, want)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

func isolateCLIConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Keep tests on the default-path code path while preventing a callers
	t.Setenv("PATTY_HOME", "")
	if err := os.Unsetenv("PATTY_HOME"); err != nil {
		t.Fatalf("unset PATTY_HOME: %v", err)
	}
	t.Setenv("PATTY_CREDENTIALS_STORE", "file")
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Chdir(t.TempDir())
	return home
}

func TestIsolateCLIConfigHomeOverridesExistingPattyCodeHome(t *testing.T) {
	externalHome := t.TempDir()
	t.Setenv("PATTY_HOME", externalHome)

	home := isolateCLIConfigHome(t)

	got := config.UserConfigPath()
	rel, err := filepath.Rel(home, got)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("UserConfigPath() = %q, outside isolated home %q", got, home)
	}
}

func TestMCPMigrationWaitsForCLIWorkspace(t *testing.T) {
	isolateCLIConfigHome(t)
	cwd := mustGetwd(t)
	if err := os.WriteFile(filepath.Join(cwd, "patty.toml"), []byte(`
[[plugins]]
name = "cwd-project"
command = "cwd-project-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	migrateLegacyConfigForCLI()
	if cfg := config.LoadForEdit(config.UserConfigPath()); hasPluginNamed(cfg, "cwd-project") {
		t.Fatalf("early CLI legacy migration imported the cwd project plugin: %+v", cfg.Plugins)
	}

	migrateMCPConfigForCLIWorkspace()
	if cfg := config.LoadForEdit(config.UserConfigPath()); !hasPluginNamed(cfg, "cwd-project") {
		t.Fatalf("workspace-aware CLI migration did not import project plugin: %+v", cfg.Plugins)
	}
}

func hasPluginNamed(cfg *config.Config, name string) bool {
	if cfg == nil {
		return false
	}
	for _, plugin := range cfg.Plugins {
		if plugin.Name == name {
			return true
		}
	}
	return false
}

func TestMetadataCommandsDoNotProbeTerminalTheme(t *testing.T) {
	defer func(prev func() (terminalRGB, bool)) { terminalProbe = prev }(terminalProbe)
	terminalProbe = func() (terminalRGB, bool) {
		t.Fatal("metadata command should not query terminal background")
		return terminalRGB{}, false
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"version"}, "test-version"); rc != 0 {
			t.Fatalf("version rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "patcode test-version") {
		t.Fatalf("version output = %q", out)
	}

	out = captureStdout(t, func() {
		if rc := Run([]string{"help"}, "test-version"); rc != 0 {
			t.Fatalf("help rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Usage:") && !strings.Contains(out, "사용법:") {
		t.Fatalf("help output missing usage:\n%s", out)
	}
	if !strings.Contains(out, "patcode run [--model NAME] [--max-steps N] [-c|--continue] [--resume PATH] [--copy] [--output-format FORMAT] <task>") {
		t.Fatalf("help output missing run resume flags:\n%s", out)
	}
}

func TestRunDispatchesACPLongFlagAlias(t *testing.T) {
	out, errOut := captureCLIOutput(t, func() {
		if rc := Run([]string{"--acp", "-h"}, "test-version"); rc != 0 {
			t.Fatalf("Run --acp -h rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Usage of acp:") {
		t.Fatalf("--acp should dispatch to the ACP command, got stdout:\n%s", out)
	}
	if errOut != "" {
		t.Fatalf("--acp help wrote stderr: %q", errOut)
	}
	if strings.Contains(out, "unknown command") {
		t.Fatalf("--acp should not be treated as an unknown command:\n%s", out)
	}
}

func TestRunDefaultsToInteractiveSession(t *testing.T) {
	isolateCLIConfigHome(t)

	prev := runInteractiveSession
	prevInteractive := cliIsInteractive
	t.Cleanup(func() {
		runInteractiveSession = prev
		cliIsInteractive = prevInteractive
	})
	cliIsInteractive = func() bool { return true }

	var gotArgs []string
	runInteractiveSession = func(args []string, _ string) int {
		gotArgs = append([]string(nil), args...)
		return 17
	}

	if rc := Run(nil, "test-version"); rc != 17 {
		t.Fatalf("Run(nil) rc = %d, want 17", rc)
	}
	if gotArgs != nil {
		t.Fatalf("interactive args = %#v, want nil", gotArgs)
	}
}

func TestRunDispatchesProfileFlagToInteractiveSession(t *testing.T) {
	isolateCLIConfigHome(t)

	prev := runInteractiveSession
	prevInteractive := cliIsInteractive
	t.Cleanup(func() {
		runInteractiveSession = prev
		cliIsInteractive = prevInteractive
	})
	cliIsInteractive = func() bool { return true }

	var gotArgs []string
	runInteractiveSession = func(args []string, _ string) int {
		gotArgs = append([]string(nil), args...)
		return 17
	}

	if rc := Run([]string{"--profile", "delivery"}, "test-version"); rc != 17 {
		t.Fatalf("Run --profile delivery rc = %d, want 17 (interactive session dispatch)", rc)
	}
	want := []string{"--profile", "delivery"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("interactive args = %#v, want %#v", gotArgs, want)
	}
}

func TestRunNoArgsNonInteractivePrintsUsage(t *testing.T) {
	isolateCLIConfigHome(t)

	prev := runInteractiveSession
	prevInteractive := cliIsInteractive
	t.Cleanup(func() {
		runInteractiveSession = prev
		cliIsInteractive = prevInteractive
	})
	cliIsInteractive = func() bool { return false }
	runInteractiveSession = func(args []string, _ string) int {
		t.Fatalf("non-interactive no-arg Run should not start session with %#v", args)
		return 99
	}

	out := captureStdout(t, func() {
		if rc := Run(nil, "test-version"); rc != 0 {
			t.Fatalf("Run(nil) rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "patcode —") || !strings.Contains(out, "patcode run") {
		t.Fatalf("non-interactive no-arg Run should print usage, got:\n%s", out)
	}
}

func TestRunRoutesBareInteractiveFlagsToSession(t *testing.T) {
	isolateCLIConfigHome(t)

	prev := runInteractiveSession
	t.Cleanup(func() { runInteractiveSession = prev })

	for _, args := range [][]string{
		{"--continue"},
		{"--continue=true"},
		{"-c"},
		{"-c=true"},
		{"--resume=true"},
		{"-r=true"},
		{"--yolo=true"},
		{"--dangerously-skip-permissions=true"},
		{"--permission-mode=plan"},
		{"--effort=max"},
	} {
		var gotArgs []string
		runInteractiveSession = func(args []string, _ string) int {
			gotArgs = append([]string(nil), args...)
			return 23
		}

		if rc := Run(args, "test-version"); rc != 23 {
			t.Fatalf("Run(%#v) rc = %d, want 23", args, rc)
		}
		if !reflect.DeepEqual(gotArgs, args) {
			t.Fatalf("interactive args = %#v, want %#v", gotArgs, args)
		}
	}
}

func TestRunReportsFlagParseErrors(t *testing.T) {
	isolateCLIConfigHome(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "run unknown flag", args: []string{"run", "--unknown"}, want: "unknown flag: --unknown"},
		{name: "run invalid value", args: []string{"run", "--max-steps=invalid"}, want: "invalid argument \"invalid\" for \"--max-steps\" flag"},
		{name: "run missing value", args: []string{"run", "--model"}, want: "flag needs an argument: --model"},
		{name: "chat unknown flag", args: []string{"chat", "--unknown"}, want: "unknown flag: --unknown"},
		{name: "serve unknown flag", args: []string{"serve", "--unknown"}, want: "flag provided but not defined: -unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := captureStderr(t, func() {
				if rc := Run(tt.args, "test-version"); rc != 2 {
					t.Fatalf("Run(%q) rc = %d, want 2", tt.args, rc)
				}
			})
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("Run(%q) stderr = %q, want %q", tt.args, stderr, tt.want)
			}
			if strings.Contains(stderr, "Usage of") {
				t.Fatalf("Run(%q) should print a concise error, got:\n%s", tt.args, stderr)
			}
		})
	}
}

func TestSubcommandHelpReturnsSuccess(t *testing.T) {
	isolateCLIConfigHome(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "run", args: []string{"run", "--help"}, want: "Usage of run:"},
		{name: "chat", args: []string{"chat", "--help"}, want: "Usage of patty:"},
		{name: "serve", args: []string{"serve", "--help"}, want: "Usage of serve:"},
		{name: "upgrade", args: []string{"upgrade", "--help"}, want: "Usage of upgrade:"},
		{name: "remote connect", args: []string{"remote", "connect", "--help"}, want: "Usage of remote connect:"},
		{name: "remote add before name", args: []string{"remote", "add", "--help"}, want: remoteAddUsage},
		{name: "remote add before target", args: []string{"remote", "add", "box", "--help"}, want: remoteAddUsage},
		{name: "remote serve before action", args: []string{"remote", "serve", "--help"}, want: remoteServeUsage},
		{name: "remote serve before name", args: []string{"remote", "serve", "start", "--help"}, want: remoteServeUsage},
		{name: "subagent create", args: []string{"subagent", "create", "--help"}, want: subagentUsageText},
		{name: "subagent edit", args: []string{"subagent", "edit", "--help"}, want: subagentUsageText},
		{name: "subagent delete", args: []string{"subagent", "delete", "--help"}, want: subagentUsageText},
		{name: "subagent try", args: []string{"subagent", "try", "--help"}, want: subagentUsageText},
		{name: "subagent run", args: []string{"subagent", "run", "--help"}, want: subagentUsageText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureCLIOutput(t, func() {
				if rc := Run(tt.args, "test-version"); rc != 0 {
					t.Fatalf("Run(%q) rc = %d, want 0", tt.args, rc)
				}
			})
			if !strings.Contains(stdout, tt.want) {
				t.Fatalf("Run(%q) help missing %q:\n%s", tt.args, tt.want, stdout)
			}
			if stderr != "" {
				t.Fatalf("Run(%q) help wrote stderr: %q", tt.args, stderr)
			}
			if strings.Contains(stdout, "help requested") {
				t.Fatalf("Run(%q) reported help as an error:\n%s", tt.args, stdout)
			}
		})
	}
}

func TestRunPrintAliasDispatchesRunFlags(t *testing.T) {
	isolateCLIConfigHome(t)
	out, errOut := captureCLIOutput(t, func() {
		if rc := Run([]string{"-p", "-h"}, "test-version"); rc != 0 {
			t.Fatalf("Run(-p -h) rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Usage of run:") {
		t.Fatalf("-p should dispatch to one-shot run flags, got:\n%s", out)
	}
	if errOut != "" {
		t.Fatalf("-p help wrote stderr: %q", errOut)
	}
}

// TestRunPrintFlagAfterLeadingFlagsDispatchesRun covers `patcode --model X -p`:
// a print flag trailing other top-level flags must still route to `run --print`,
func TestRunPrintFlagAfterLeadingFlagsDispatchesRun(t *testing.T) {
	isolateCLIConfigHome(t)
	prev := runInteractiveSession
	t.Cleanup(func() { runInteractiveSession = prev })
	runInteractiveSession = func([]string, string) int {
		t.Fatal("print flag after leading flags must not route to the interactive session")
		return 0
	}
	out, errOut := captureCLIOutput(t, func() {
		if rc := Run([]string{"--model", "x", "-p", "-h"}, "test-version"); rc != 0 {
			t.Fatalf("Run(--model x -p -h) rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Usage of run:") {
		t.Fatalf("--model x -p should dispatch to one-shot run flags, got:\n%s", out)
	}
	if errOut != "" {
		t.Fatalf("--model x -p help wrote stderr: %q", errOut)
	}
}

func TestParsePermissionModeClaudeAliases(t *testing.T) {
	tests := map[string]cliPermissionMode{
		"ask":               {approval: control.ToolApprovalAsk},
		"manual":            {approval: control.ToolApprovalAsk},
		"acceptEdits":       {approval: control.ToolApprovalAsk, allow: []string{"write_file", "edit_file", "multi_edit", "move_file", "notebook_edit", "delete_range", "delete_symbol"}},
		"dontAsk":           {approval: control.ToolApprovalDontAsk},
		"plan":              {approval: control.ToolApprovalAsk, plan: true},
		"bypassPermissions": {approval: control.ToolApprovalYolo},
	}
	for input, want := range tests {
		got, err := parsePermissionMode(input)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("parsePermissionMode(%q) = (%+v, %v), want %+v", input, got, err, want)
		}
	}
}

func TestResolveRunPermissionModeRequiresExplicitAuto(t *testing.T) {
	if got, err := resolveRunPermissionMode("ask", false, false); err != nil || got != "ask" {
		t.Fatalf("default run permission mode = (%q, %v), want ask", got, err)
	}
	if got, err := resolveRunPermissionMode("ask", true, false); err != nil || got != "auto" {
		t.Fatalf("-y run permission mode = (%q, %v), want auto", got, err)
	}
	if got, err := resolveRunPermissionMode("dontAsk", true, true); err == nil || got != "" {
		t.Fatalf("combined permission flags = (%q, %v), want conflict", got, err)
	}
}

func TestRunKeepsChatAndCodeCompatibilityAliases(t *testing.T) {
	isolateCLIConfigHome(t)

	prev := runInteractiveSession
	t.Cleanup(func() { runInteractiveSession = prev })

	var calls [][]string
	runInteractiveSession = func(args []string, _ string) int {
		calls = append(calls, append([]string(nil), args...))
		return 0
	}

	if rc := Run([]string{"chat", "--resume"}, "test-version"); rc != 0 {
		t.Fatalf("Run(chat --resume) rc = %d, want 0", rc)
	}
	if rc := Run([]string{"code", "--continue"}, "test-version"); rc != 0 {
		t.Fatalf("Run(code --continue) rc = %d, want 0", rc)
	}

	want := [][]string{{"--resume"}, {"--continue"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("interactive calls = %#v, want %#v", calls, want)
	}
}

func TestRunMigratesLegacyConfigBeforeConfigOnlyCommands(t *testing.T) {
	isolateCLIConfigHome(t)
	legacyPath := filepath.Join(filepath.Dir(config.UserConfigPath()), "patty.toml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`
default_model = "deepseek-flash"

[[plugins]]
name = "legacy-cli"
command = "legacy-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"mcp", "list"}, "test-version"); rc != 0 {
			t.Fatalf("mcp list rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "legacy-cli") {
		t.Fatalf("mcp list should include migrated legacy config:\n%s", out)
	}

	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read migrated user config: %v", err)
	}
	for _, want := range []string{`config_version = 5`, `[desktop]`, `name    = "legacy-cli"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("migrated config missing %q:\n%s", want, body)
		}
	}
}

func TestRunAppliesUserConfigUpgradesOnStartup(t *testing.T) {
	isolateCLIConfigHome(t)
	path := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("config_version = 2\ndefault_model = \"deepseek-flash\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		if rc := Run([]string{"mcp", "list"}, "test-version"); rc != 0 {
			t.Fatalf("mcp list rc = %d, want 0", rc)
		}
	})

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read upgraded user config: %v", err)
	}
	if !strings.Contains(string(body), "config_version = 5") {
		t.Fatalf("CLI startup should apply user config upgrades:\n%s", body)
	}
}

func TestRunMetadataCommandsDoNotMigrateLegacyConfig(t *testing.T) {
	isolateCLIConfigHome(t)
	legacyPath := filepath.Join(filepath.Dir(config.UserConfigPath()), "patty.toml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`default_model = "deepseek-flash"`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"version"}, "test-version"); rc != 0 {
			t.Fatalf("version rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "patcode test-version") {
		t.Fatalf("version output = %q", out)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("version should not migrate legacy config, stat err=%v", err)
	}
}

func TestConfigLoadIgnoresRetiredAutoPlan(t *testing.T) {
	isolateCLIConfigHome(t)
	if err := os.WriteFile("patty.toml", []byte("[agent]\nauto_plan = \"on\"\nauto_plan_classifier = \"deepseek-flash\"\n"), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Agent.AutoPlan != "off" || cfg.Agent.AutoPlanClassifier != "" {
		t.Fatalf("retired auto-plan config = (%q, %q), want off/empty", cfg.Agent.AutoPlan, cfg.Agent.AutoPlanClassifier)
	}
}

func TestConfigAutoPlanCompatibilityCommandKeepsOffAsNoOp(t *testing.T) {
	isolateCLIConfigHome(t)
	path := config.UserConfigPath()
	cfg := config.Default()
	cfg.Agent.Temperature = 0.4
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user config before command: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "auto-plan", "off"}, "test-version"); rc != 0 {
			t.Fatalf("config auto-plan off rc = %d, want 0", rc)
		}
	})
	if out != "auto_plan = \"off\"\n" {
		t.Fatalf("config auto-plan off output = %q", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user config after command: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("config auto-plan off must not rewrite user config\nbefore:\n%s\nafter:\n%s", before, after)
	}

	out = captureStdout(t, func() {
		if rc := Run([]string{"config", "auto-plan"}, "test-version"); rc != 0 {
			t.Fatalf("config auto-plan query rc = %d, want 0", rc)
		}
	})
	if out != "auto_plan = \"off\"\n" {
		t.Fatalf("config auto-plan query output = %q", out)
	}
}

func TestConfigAutoPlanCompatibilityCommandRejectsEnable(t *testing.T) {
	isolateCLIConfigHome(t)

	errOut := captureStderr(t, func() {
		if rc := Run([]string{"config", "auto-plan", "on"}, "test-version"); rc != 2 {
			t.Fatalf("config auto-plan on rc = %d, want 2", rc)
		}
	})
	if !strings.Contains(errOut, "automatic plan mode has been retired") {
		t.Fatalf("config auto-plan on stderr = %q", errOut)
	}
}

func TestConfigReasoningLanguageCommandWritesUserConfig(t *testing.T) {
	isolateCLIConfigHome(t)

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "reasoning-language", "ko-KR"}, "test-version"); rc != 0 {
			t.Fatalf("config reasoning-language rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, `reasoning_language = "ko-KR"`) {
		t.Fatalf("config reasoning-language output = %q", out)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Agent.ReasoningLanguage != "ko-KR" || cfg.ReasoningLanguage() != "ko-KR" {
		t.Fatalf("saved reasoning_language = %q/%q, want ko-KR", cfg.Agent.ReasoningLanguage, cfg.ReasoningLanguage())
	}
}

func TestConfigReasoningLanguageLocalCreatesMinimalProjectOverride(t *testing.T) {
	isolateCLIConfigHome(t)

	userCfg := config.Default()
	userCfg.DefaultModel = "mimo-pro"
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "reasoning-language", "--local", "en"}, "test-version"); rc != 0 {
			t.Fatalf("config reasoning-language --local rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, `reasoning_language = "en"`) {
		t.Fatalf("config reasoning-language --local output = %q", out)
	}

	body, err := os.ReadFile("patty.toml")
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}
	if strings.Contains(string(body), "default_model") {
		t.Fatalf("project reasoning-language override should not pin default_model:\n%s", body)
	}
	if !strings.Contains(string(body), "[agent]") || !strings.Contains(string(body), `reasoning_language = "en"`) {
		t.Fatalf("project config missing reasoning_language override:\n%s", body)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}
	if cfg.DefaultModel != "mimo-pro" {
		t.Fatalf("default_model = %q, want global mimo-pro", cfg.DefaultModel)
	}
	if cfg.ReasoningLanguage() != "en" {
		t.Fatalf("reasoning_language = %q, want local en", cfg.ReasoningLanguage())
	}
}

func TestConfigReasoningLanguageRejectsAliases(t *testing.T) {
	isolateCLIConfigHome(t)

	errOut := captureStderr(t, func() {
		if rc := Run([]string{"config", "reasoning-language", "fr"}, "test-version"); rc != 2 {
			t.Fatalf("config reasoning-language alias rc = %d, want 2", rc)
		}
	})
	if !strings.Contains(errOut, "must be auto|ko-KR|en") {
		t.Fatalf("config reasoning-language alias stderr = %q", errOut)
	}
}

func TestConfigCompactRatioCommandWritesUserConfigAndReportsSource(t *testing.T) {
	isolateCLIConfigHome(t)
	userCfg := config.Default()
	userCfg.Agent.Temperature = 0.42
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "compact-ratio", "95.9"}, "test-version"); rc != 0 {
			t.Fatalf("config compact-ratio rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "compact_ratio = 95.9%") || !strings.Contains(out, "user:") {
		t.Fatalf("config compact-ratio output = %q", out)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if got := cfg.Agent.CompactRatio; math.Abs(got-0.959) > 1e-12 {
		t.Fatalf("saved compact ratio = %v, want 0.959", got)
	}
	if got := cfg.Agent.Temperature; got != 0.42 {
		t.Fatalf("compact-ratio update changed temperature to %v, want 0.42", got)
	}

	out = captureStdout(t, func() {
		if rc := Run([]string{"config", "compact-ratio"}, "test-version"); rc != 0 {
			t.Fatalf("config compact-ratio query rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "compact_ratio = 95.9%") || !strings.Contains(out, "user:") {
		t.Fatalf("config compact-ratio query output = %q", out)
	}
}

func TestConfigCompactRatioQueryReportsBuiltInDefault(t *testing.T) {
	isolateCLIConfigHome(t)

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "compact-ratio"}, "test-version"); rc != 0 {
			t.Fatalf("config compact-ratio query rc = %d, want 0", rc)
		}
	})
	if out != "compact_ratio = 95.97% (built-in default)\n" {
		t.Fatalf("config compact-ratio query output = %q", out)
	}
}

func TestConfigCompactRatioLocalCreatesMinimalProjectOverride(t *testing.T) {
	isolateCLIConfigHome(t)

	userCfg := config.Default()
	userCfg.DefaultModel = "mimo-pro"
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := Run([]string{"config", "compact-ratio", "--local", "70"}, "test-version"); rc != 0 {
			t.Fatalf("config compact-ratio --local rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "compact_ratio = 70%") || !strings.Contains(out, "project:") {
		t.Fatalf("config compact-ratio --local output = %q", out)
	}

	body, err := os.ReadFile("patty.toml")
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}
	if strings.Contains(string(body), "default_model") {
		t.Fatalf("project compact-ratio override should not pin default_model:\n%s", body)
	}
	if !strings.Contains(string(body), "[agent]") || !strings.Contains(string(body), "compact_ratio = 0.7") {
		t.Fatalf("project config missing compact_ratio override:\n%s", body)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}
	if cfg.DefaultModel != "mimo-pro" {
		t.Fatalf("default_model = %q, want global mimo-pro", cfg.DefaultModel)
	}
	if cfg.Agent.CompactRatio != 0.7 {
		t.Fatalf("compact ratio = %v, want local 0.7", cfg.Agent.CompactRatio)
	}

	out = captureStdout(t, func() {
		if rc := Run([]string{"config", "compact-ratio"}, "test-version"); rc != 0 {
			t.Fatalf("config compact-ratio query rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "compact_ratio = 70%") || !strings.Contains(out, "project:") {
		t.Fatalf("project compact-ratio query output = %q", out)
	}
}

func TestConfigCompactRatioLocalHonorsInheritedUserThresholds(t *testing.T) {
	tests := []struct {
		name      string
		ratio     string
		configure func(*config.Config)
	}{
		{
			name:  "force ratio",
			ratio: "95",
			configure: func(cfg *config.Config) {
				cfg.Agent.CompactRatio = 0.8
				cfg.Agent.CompactForceRatio = 0.9
			},
		},
		{
			name:  "tool result snip ratio",
			ratio: "70",
			configure: func(cfg *config.Config) {
				cfg.Agent.CompactRatio = 0.85
				cfg.Agent.ToolResultSnipRatio = 0.8
			},
		},
	}
	for _, test := range tests {
		for _, existingProject := range []bool{false, true} {
			projectState := "new project"
			if existingProject {
				projectState = "existing project"
			}
			t.Run(test.name+"/"+projectState, func(t *testing.T) {
				isolateCLIConfigHome(t)
				userCfg := config.Default()
				test.configure(userCfg)
				if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
					t.Fatalf("write user config: %v", err)
				}

				var before []byte
				if existingProject {
					before = []byte("[agent]\ntemperature = 0.42\n")
					if err := os.WriteFile("patty.toml", before, 0o600); err != nil {
						t.Fatalf("write project config: %v", err)
					}
				}

				errOut := captureStderr(t, func() {
					if rc := Run([]string{"config", "compact-ratio", "--local", test.ratio}, "test-version"); rc != 2 {
						t.Fatalf("config compact-ratio --local %s rc = %d, want 2", test.ratio, rc)
					}
				})
				if !strings.Contains(errOut, "compact ratio") {
					t.Fatalf("config compact-ratio --local stderr = %q", errOut)
				}

				after, err := os.ReadFile("patty.toml")
				if !existingProject {
					if !os.IsNotExist(err) {
						t.Fatalf("rejected local override created project config, err=%v body=%q", err, after)
					}
					return
				}
				if err != nil {
					t.Fatalf("read existing project config: %v", err)
				}
				if string(after) != string(before) {
					t.Fatalf("rejected local override changed project config:\n%s", after)
				}
			})
		}
	}
}

func TestConfigCompactRatioRejectsValuesOutsideEditableRange(t *testing.T) {
	isolateCLIConfigHome(t)

	for _, value := range []string{"64", "98", "NaN", "+Inf", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			errOut := captureStderr(t, func() {
				if rc := Run([]string{"config", "compact-ratio", value}, "test-version"); rc != 2 {
					t.Fatalf("config compact-ratio %s rc = %d, want 2", value, rc)
				}
			})
			if !strings.Contains(errOut, "finite percentage") && !strings.Contains(errOut, "between 0.65 and 0.97") {
				t.Fatalf("config compact-ratio %s stderr = %q", value, errOut)
			}
		})
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("invalid compact ratio wrote user config, stat err=%v", err)
	}
}




func TestProvidersWithMissingKeysOnlyChecksActiveDefaultModel(t *testing.T) {
	isolateCLIConfigHome(t)
	cfg := config.Default()
	t.Setenv("DARI_HARNESS_KEY", "")
	t.Setenv("MIMO_API_KEY", "")

	missing := providersWithMissingKeys(cfg)
	if len(missing) != 1 {
		t.Fatalf("missing providers = %+v, want only active default model provider", missing)
	}
	if missing[0].APIKeyEnv != "DARI_HARNESS_KEY" {
		t.Fatalf("missing key env = %q, want DARI_HARNESS_KEY", missing[0].APIKeyEnv)
	}
}

func TestProvidersWithMissingKeysIgnoresInheritedEnvironment(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv("DARI_HARNESS_KEY", "inherited-only")

	missing := providersWithMissingKeys(config.Default())
	if len(missing) != 1 || missing[0].APIKeyEnv != "DARI_HARNESS_KEY" {
		t.Fatalf("inherited process key suppressed missing-store prompt: %+v", missing)
	}
}

func TestProvidersWithMissingKeysIgnoresUnusedBuiltInPresets(t *testing.T) {
	isolateCLIConfigHome(t)
	cfg := config.Default()
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv("MIMO_API_KEY", "")

	if missing := providersWithMissingKeys(cfg); len(missing) != 0 {
		t.Fatalf("missing providers = %+v, want none when only the configured default is keyed", missing)
	}
}

func TestProvidersWithMissingKeysIncludesReferencedSecondaryModels(t *testing.T) {
	isolateCLIConfigHome(t)
	cfg := config.Default()
	cfg.Providers = append(cfg.Providers,
		config.ProviderEntry{Name: "mimo-pro", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5-pro", APIKeyEnv: "MIMO_API_KEY"},
		config.ProviderEntry{Name: "mimo-flash", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5", APIKeyEnv: "MIMO_API_KEY"},
	)
	cfg.Agent.PlannerModel = "mimo-pro"
	cfg.Agent.SubagentModel = "mimo-flash"
	cfg.Agent.SubagentModels = map[string]string{
		"review": "mimo-pro/mimo-v2.5-pro",
	}
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv("MIMO_API_KEY", "")

	missing := providersWithMissingKeys(cfg)
	if len(missing) != 1 {
		t.Fatalf("missing providers = %+v, want MiMo once", missing)
	}
	if missing[0].APIKeyEnv != "MIMO_API_KEY" {
		t.Fatalf("missing key env = %q, want MIMO_API_KEY", missing[0].APIKeyEnv)
	}
}

type cliRecordSink struct {
	events []event.Kind
}

func (s *cliRecordSink) Emit(e event.Event) {
	s.events = append(s.events, e.Kind)
}

type cliRecordSender struct {
	messages []notify.Message
}

func (s *cliRecordSender) Send(m notify.Message) error {
	s.messages = append(s.messages, m)
	return nil
}

func TestLanguageChoicesPutKoreanFirstForNormalizedDefault(t *testing.T) {
	items, tags := languageChoices("ko")
	if len(items) != 2 || len(tags) != 2 || items[0].name != "한국어" || tags[0] != "ko-KR" {
		t.Fatalf("Korean choices = %#v / %#v, want Korean first", items, tags)
	}

	items, tags = languageChoices("en")
	if items[0].name != "English" || tags[0] != "en" {
		t.Fatalf("English choices = %#v / %#v, want explicit English first", items, tags)
	}
}

func TestWithNotificationsWrapsCLISinkWithConfiguredSender(t *testing.T) {
	inner := &cliRecordSink{}
	sender := &cliRecordSender{}
	calls := 0
	prev := newNotificationSender
	newNotificationSender = func() notify.Sender {
		calls++
		return sender
	}
	t.Cleanup(func() { newNotificationSender = prev })

	cfg := config.Default()
	cfg.Notifications.Enabled = true

	wrapped := withNotifications(inner, cfg)
	wrapped.Emit(event.Event{Kind: event.TurnDone})

	if calls != 1 {
		t.Fatalf("newNotificationSender calls = %d, want 1", calls)
	}
	if len(inner.events) != 1 || inner.events[0] != event.TurnDone {
		t.Fatalf("forwarded events = %v, want [TurnDone]", inner.events)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("notifications = %d, want 1", len(sender.messages))
	}
	if sender.messages[0].Body != "Turn finished" {
		t.Fatalf("notification body = %q, want Turn finished", sender.messages[0].Body)
	}
}

func TestConfigTelemetryCommandRoundTripAndOptOutCleanup(t *testing.T) {
	isolateCLIConfigHome(t)
	out := captureStdout(t, func() {
		if rc := configTelemetryCommand(nil); rc != 0 {
			t.Fatalf("config telemetry query rc = %d", rc)
		}
	})
	if !strings.Contains(out, `cli_metrics = "auto"`) {
		t.Fatalf("default telemetry query = %q", out)
	}
	if rc := configTelemetryCommand([]string{"on"}); rc != 0 {
		t.Fatalf("config telemetry on rc = %d", rc)
	}
	cfg, err := config.Load()
	if err != nil || cfg.CLITelemetryMode() != "on" {
		t.Fatalf("saved telemetry mode = %q, err = %v", cfg.CLITelemetryMode(), err)
	}
	pending := filepath.Join(config.PattyHomeDir(), "cli-telemetry-pending")
	if err := os.MkdirAll(pending, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pending, "pending.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc := configTelemetryCommand([]string{"off"}); rc != 0 {
		t.Fatalf("config telemetry off rc = %d", rc)
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("opt-out did not remove pending queue: %v", err)
	}
}

func TestConfigTelemetryCommandReportsOptOutCleanupFailure(t *testing.T) {
	isolateCLIConfigHome(t)
	previous := cleanupCLITelemetry
	t.Cleanup(func() { cleanupCLITelemetry = previous })
	cleanupCLITelemetry = func(string) error { return errors.New("cleanup denied") }

	errOut := captureStderr(t, func() {
		if rc := configTelemetryCommand([]string{"off"}); rc != 1 {
			t.Fatalf("config telemetry off rc = %d, want 1", rc)
		}
	})
	if !strings.Contains(errOut, "telemetry disabled") || !strings.Contains(errOut, "cleanup denied") {
		t.Fatalf("cleanup failure stderr = %q", errOut)
	}
	cfg, err := config.Load()
	if err != nil || cfg.CLITelemetryMode() != "off" {
		t.Fatalf("saved telemetry mode = %q, err = %v", cfg.CLITelemetryMode(), err)
	}
}

func TestConfiguredCLITelemetryDoesNotPromptAgain(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)
	previousSave := persistCLITelemetryConsent
	previousStart := startCLITelemetryReporter
	t.Cleanup(func() {
		persistCLITelemetryConsent = previousSave
		startCLITelemetryReporter = previousStart
	})
	persistCalls := 0
	persistCLITelemetryConsent = func(string) error {
		persistCalls++
		return nil
	}
	want := &telemetry.Reporter{}
	startCalls := 0
	startCLITelemetryReporter = func(opts telemetry.Options) *telemetry.Reporter {
		startCalls++
		if telemetry.Enabled(opts.Mode, opts.Version, opts.Interactive) {
			return want
		}
		return nil
	}

	for _, mode := range []string{"auto", "on", "off"} {
		cfg := config.Default()
		if err := cfg.SetCLITelemetryMode(mode); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		got := startCLITelemetryWithIO(cfg, telemetry.Options{
			Version: "v1.20.0", Interactive: true, CLIMode: "tui",
		}, strings.NewReader("n\n"), &out, io.Discard)
		if out.Len() != 0 {
			t.Fatalf("configured mode %q prompted again: %q", mode, out.String())
		}
		if mode == "off" && got != nil {
			t.Fatalf("configured off returned reporter %p", got)
		}
		if mode != "off" && got != want {
			t.Fatalf("configured %s returned %p, want %p", mode, got, want)
		}
	}
	if persistCalls != 0 || startCalls != 3 {
		t.Fatalf("configured modes persisted=%d started=%d, want 0 and 3", persistCalls, startCalls)
	}
}

func TestUndecidedCLITelemetryDoesNotPromptOrUploadWhenIneligible(t *testing.T) {
	for _, tc := range []struct {
		name        string
		version     string
		interactive bool
		envKey      string
		envValue    string
	}{
		{name: "noninteractive", version: "v1.20.0"},
		{name: "development", version: "dev", interactive: true},
		{name: "CI", version: "v1.20.0", interactive: true, envKey: "CI", envValue: "1"},
		{name: "do not track", version: "v1.20.0", interactive: true, envKey: "DO_NOT_TRACK", envValue: "1"},
		{name: "environment opt out", version: "v1.20.0", interactive: true, envKey: "PATTY_TELEMETRY", envValue: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateCLIConfigHome(t)
			clearCLITelemetryPolicyEnv(t)
			if tc.envKey != "" {
				t.Setenv(tc.envKey, tc.envValue)
			}
			cfg, err := config.LoadForRootReadOnly(".")
			if err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			if got := startCLITelemetryWithIO(cfg, telemetry.Options{
				Version: tc.version, Interactive: tc.interactive, CLIMode: "tui",
			}, strings.NewReader("\n"), &out, &errOut); got != nil {
				t.Fatalf("ineligible telemetry returned reporter %p", got)
			}
			if out.Len() != 0 || errOut.Len() != 0 {
				t.Fatalf("ineligible telemetry wrote output: stdout=%q stderr=%q", out.String(), errOut.String())
			}
			if _, err := os.Stat(config.UserConfigPath()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("ineligible invocation wrote config: %v", err)
			}
		})
	}
}

func TestLegacySafeModeEnvDoesNotAlterConfiguredCLITelemetry(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)
	t.Setenv("PATTY_SAFE_MODE", "1")
	cfg := config.Default()
	if err := cfg.SetCLITelemetryMode("auto"); err != nil {
		t.Fatal(err)
	}
	previousStart := startCLITelemetryReporter
	t.Cleanup(func() { startCLITelemetryReporter = previousStart })
	want := &telemetry.Reporter{}
	startCLITelemetryReporter = func(telemetry.Options) *telemetry.Reporter { return want }
	if got := startCLITelemetryWithIO(cfg, telemetry.Options{
		Version: "v1.20.0", Interactive: true, CLIMode: "tui",
	}, strings.NewReader(""), io.Discard, io.Discard); got != want {
		t.Fatalf("telemetry reporter = %p, want %p", got, want)
	}
}

func clearCLITelemetryPolicyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DO_NOT_TRACK", "PATTY_TELEMETRY", "PATTY_SAFE_MODE", "CI", "CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI", "JENKINS_URL",
		"TEAMCITY_VERSION", "TF_BUILD",
	} {
		t.Setenv(key, "")
	}
}

func TestSetupOverwritePromptShowsYNDefault(t *testing.T) {
	t.Cleanup(func() { i18n.DetectLanguage("en") })
	for _, lang := range []string{"en", "ko-KR"} {
		i18n.DetectLanguage(lang)
		var out bytes.Buffer
		if confirmReconfigureExistingConfig("config.toml", bufio.NewScanner(strings.NewReader("\n")), &out) {
			t.Fatalf("%s empty overwrite answer should keep existing config", lang)
		}
		if !strings.Contains(out.String(), "[y/N]:") {
			t.Fatalf("%s overwrite prompt should show explicit [y/N] default, got %q", lang, out.String())
		}
	}
}

// TestConfigureKeys verifies that a shared api_key_env (each vendors SKUs use
func TestConfigureKeys(t *testing.T) {
	// by the new "reuse existing" path and the prompt would be skipped,
	t.Setenv("DARI_HARNESS_KEY", "")

	selected := config.Default().Providers

	input := "patty-key\n"
	env := configureKeys(selected, strings.NewReader(input), io.Discard)

	if len(env) != 1 {
		t.Fatalf("env = %v (want 1: Patty asked once)", env)
	}
	if env[0] != "DARI_HARNESS_KEY=patty-key" {
		t.Errorf("env[0] = %q", env[0])
	}
}

// TestConfigureKeysReusesExistingEnv covers the user already typed the key
// in the URL-fetch flow, don't ask again" path. When the env var is set
// must NOT consume from the input stream — otherwise the user's next typed
// line bleeds into the next providers prompt. It also must include the
func TestConfigureKeysReusesExistingEnv(t *testing.T) {
	t.Setenv("DARI_HARNESS_KEY", "preset-patty-key")

	selected := config.Default().Providers
	var output bytes.Buffer
	env := configureKeys(selected, strings.NewReader("\n"), &output)

	if len(env) != 1 {
		t.Fatalf("env = %v (want 1: Patty reused)", env)
	}
	if env[0] != "DARI_HARNESS_KEY=preset-patty-key" {
		t.Errorf("env[0] = %q, want re-pinned existing value", env[0])
	}
	if !strings.Contains(output.String(), "DARI_HARNESS_KEY") {
		t.Errorf("expected a 'reusing' confirmation for DARI_HARNESS_KEY, got:\n%s", output.String())
	}
}

func TestConfigureKeysCanResetExistingEnv(t *testing.T) {
	t.Setenv("DARI_HARNESS_KEY", "stale-patty-key")

	selected := config.Default().Providers
	var output bytes.Buffer
	env := configureKeys(selected, strings.NewReader("y\nfresh-patty-key\n"), &output)

	if len(env) != 1 {
		t.Fatalf("env = %v (want 1: Patty reset)", env)
	}
	if env[0] != "DARI_HARNESS_KEY=fresh-patty-key" {
		t.Errorf("env[0] = %q, want freshly entered value", env[0])
	}
	if !strings.Contains(output.String(), "[y/N]:") || !strings.Contains(output.String(), "DARI_HARNESS_KEY") {
		t.Errorf("expected a reset confirmation for DARI_HARNESS_KEY, got:\n%s", output.String())
	}
}

func TestConfigureKeysAllSetDefaultsToReusingInput(t *testing.T) {
	t.Setenv("DARI_HARNESS_KEY", "patty")

	selected := config.Default().Providers
	env := configureKeys(selected, strings.NewReader("\n"), io.Discard)
	if len(env) != 1 {
		t.Errorf("env = %v, want 1 (Patty reused)", env)
	}
}

func TestAppendEnvUpsertReplacesExistingKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "") // also covers the os.Setenv pin path
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("# initial\nDEEPSEEK_API_KEY=stale\nMIMO_API_KEY=keepme\n"), 0o600)

	if err := appendEnv(p, []string{"DEEPSEEK_API_KEY=fresh"}); err != nil {
		t.Fatalf("appendEnv: %v", err)
	}
	got, _ := os.ReadFile(p)
	want := "# initial\nMIMO_API_KEY=keepme\nDEEPSEEK_API_KEY=fresh\n"
	if string(got) != want {
		t.Errorf("after upsert =\n%s\nwant =\n%s", got, want)
	}
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "fresh" {
		t.Errorf("process env DEEPSEEK_API_KEY = %q, want %q (upsert should pin in-process)", got, "fresh")
	}
}

// TestAppendEnvUpsertHandlesExportPrefix proves `export FOO=...` style lines
func TestAppendEnvUpsertHandlesExportPrefix(t *testing.T) {
	t.Setenv("FOO", "")
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("export FOO=old\nKEEP=yes\n"), 0o600)
	if err := appendEnv(p, []string{"FOO=new"}); err != nil {
		t.Fatalf("appendEnv: %v", err)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), "FOO=new") || strings.Contains(string(got), "FOO=old") {
		t.Errorf("export-prefixed line not replaced:\n%s", got)
	}
}

// "patty" (medium), preserving the order each family first appears in.
func TestGroupByFamily(t *testing.T) {
	order, members, info := groupByFamily(config.Default().Providers)

	if got := order; !reflect.DeepEqual(got, []string{"patty"}) {
		t.Fatalf("family order = %v, want [patty]", got)
	}
	if got := members["patty"]; !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("patty members = %v, want [0]", got)
	}
	if info["patty"].name != "Patty" {
		t.Errorf("display name = %q", info["patty"].name)
	}
}

// TestFetchOrFallbackLiveReturns covers the happy path: a live models call
// succeeds and its result wins over the preset's static list. We can't run
func TestFetchOrFallback(t *testing.T) {
	t.Run("empty base URL returns static list", func(t *testing.T) {
		probe := config.ProviderEntry{
			BaseURL: "",
			Models:  []string{"preset-a", "preset-b"},
		}
		got := fetchOrFallback(&probe, "Test")
		if !reflect.DeepEqual(got, []string{"preset-a", "preset-b"}) {
			t.Errorf("got %v, want preset-a/b", got)
		}
	})

	t.Run("no key set returns static list (offline first-run)", func(t *testing.T) {
		t.Setenv("PATTY_FETCH_TEST_KEY", "")
		probe := config.ProviderEntry{
			BaseURL:   "http://127.0.0.1:1", // unreachable, no listener
			APIKeyEnv: "PATTY_FETCH_TEST_KEY",
			Models:    []string{"preset-a"},
		}
		got := fetchOrFallback(&probe, "Test")
		if !reflect.DeepEqual(got, []string{"preset-a"}) {
			t.Errorf("got %v, want preset-a", got)
		}
	})
}

// TestFetchModelListCompatWalksCandidates covers the wizards custom-provider
// model probe. Previously the probe was a single URL (baseURL+"/models"),
// which worked for OpenAI vendors with a v1 base URL but silently failed
// for Anthropic-style root URLs (no v1) and Anthropic-compatible proxies
// (a /v1 base URL but a /v1/messages endpoint). The new helper walks
// BuildModelFetchURLs's candidate list — root + /v1 + known compat
// suffixes  so the same probe now succeeds for both shapes, matching
func TestFetchModelListCompatWalksCandidates(t *testing.T) {
	t.Run("anthropic root form resolves via v1 fallback", func(t *testing.T) {
		var gotPath atomic.Value
		gotPath.Store("")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath.Store(r.URL.Path)
			if r.URL.Path == "/v1/models" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"id":"claude-test"}]}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		models, err := fetchModelListCompat(context.Background(), srv.URL, "k")
		if err != nil {
			t.Fatalf("fetchModelListCompat: %v", err)
		}
		if !reflect.DeepEqual(models, []string{"claude-test"}) {
			t.Errorf("models = %v, want [claude-test]", models)
		}
		if got := gotPath.Load().(string); got != "/v1/models" {
			t.Errorf("probe path = %q, want /v1/models (root form should fall through to v1 candidate)", got)
		}
	})

	t.Run("versioned v1 base URL hits models directly", func(t *testing.T) {
		var gotPath atomic.Value
		gotPath.Store("")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath.Store(r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"id":"model-a"}]}`)
		}))
		defer srv.Close()

		models, err := fetchModelListCompat(context.Background(), srv.URL+"/v1", "k")
		if err != nil {
			t.Fatalf("fetchModelListCompat: %v", err)
		}
		if !reflect.DeepEqual(models, []string{"model-a"}) {
			t.Errorf("models = %v, want [model-a]", models)
		}
		if got := gotPath.Load().(string); got != "/v1/models" {
			t.Errorf("probe path = %q, want /v1/models", got)
		}
	})

	t.Run("endpoint-miss on every candidate returns empty (manual flow)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		models, err := fetchModelListCompat(context.Background(), srv.URL, "k")
		if err != nil {
			t.Fatalf("expected graceful empty result on all-miss, got err: %v", err)
		}
		if len(models) != 0 {
			t.Errorf("expected empty models on all-miss, got %v", models)
		}
	})

	t.Run("non-404 network error short-circuits with the real error", func(t *testing.T) {
		// Point at a closed port  connection refused, not a 404.
		models, err := fetchModelListCompat(context.Background(), "http://127.0.0.1:1", "k")
		if err == nil {
			t.Fatalf("expected error for unreachable host, got models=%v", models)
		}
	})
}

// family (the flash + pro SKUs), not just the first  the regression that left
// users with only flash when the live models probe failed.
func TestFamilyStaticModels(t *testing.T) {
	providers := []config.ProviderEntry{
		{Name: "deepseek-flash", Model: "deepseek-v4-flash"},
		{Name: "deepseek-pro", Model: "deepseek-v4-pro"},
		{Name: "mimo-flash", Model: "mimo-v2.5"},
	}
	got := familyStaticModels(providers, []int{0, 1})
	want := []string{"deepseek-v4-flash", "deepseek-v4-pro"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFamilyStaticModelsDedupes(t *testing.T) {
	providers := []config.ProviderEntry{
		{Name: "a", Models: []string{"x", "y"}},
		{Name: "b", Models: []string{"y", "z"}},
	}
	got := familyStaticModels(providers, []int{0, 1})
	if !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Errorf("got %v, want x/y/z deduped", got)
	}
}

// would bill pro at flashs rate.
func TestBuildFamilyEntriesSplitsPricing(t *testing.T) {
	flash := config.ProviderEntry{Name: "deepseek-flash", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", Price: &provider.Pricing{Input: 1, Output: 2}}
	pro := config.ProviderEntry{Name: "deepseek-pro", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", Price: &provider.Pricing{Input: 3, Output: 6}}
	got := buildFamilyEntries(flash, []config.ProviderEntry{flash, pro}, []string{"deepseek-v4-flash", "deepseek-v4-pro"})
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	byName := map[string]config.ProviderEntry{}
	for _, e := range got {
		byName[e.Name] = e
	}
	if e := byName["deepseek-flash"]; e.Model != "deepseek-v4-flash" || e.Price == nil || e.Price.Output != 2 {
		t.Errorf("flash entry wrong: %+v (price %+v)", e, e.Price)
	}
	if e := byName["deepseek-pro"]; e.Model != "deepseek-v4-pro" || e.Price == nil || e.Price.Output != 6 {
		t.Errorf("pro entry wrong: %+v (price %+v)", e, e.Price)
	}
}

func TestBuildFamilyEntriesUnknownModelUsesProbe(t *testing.T) {
	flash := config.ProviderEntry{Name: "deepseek-flash", Model: "deepseek-v4-flash", Price: &provider.Pricing{Input: 1}}
	got := buildFamilyEntries(flash, []config.ProviderEntry{flash}, []string{"deepseek-v4-flash", "deepseek-v9-experimental"})
	if len(got) != 1 || got[0].Name != "deepseek-flash" {
		t.Fatalf("got %+v, want one deepseek-flash entry", got)
	}
	if !reflect.DeepEqual(got[0].Models, []string{"deepseek-v4-flash", "deepseek-v9-experimental"}) {
		t.Errorf("Models = %v, want both under the probe entry", got[0].Models)
	}
}

// - The selected models land in the entrys Models field, with Model
// - A preset Default that points to a model the user didnt pick is
func TestBuildFamilyEntry(t *testing.T) {
	t.Run("default reset when not in selection", func(t *testing.T) {
		probe := config.ProviderEntry{
			Name: "deepseek", Kind: "openai",
			BaseURL: "https://api.deepseek.com",
			Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
			Default: "deepseek-v4-pro",
		}
		got := buildFamilyEntry(probe, []string{"deepseek-v4-flash"})
		if got.Model != "deepseek-v4-flash" {
			t.Errorf("Model = %q, want deepseek-v4-flash", got.Model)
		}
		if got.Default != "deepseek-v4-flash" {
			t.Errorf("Default = %q, want reset to first selected", got.Default)
		}
		if !reflect.DeepEqual(got.Models, []string{"deepseek-v4-flash"}) {
			t.Errorf("Models = %v", got.Models)
		}
		if got.BaseURL != "https://api.deepseek.com" {
			t.Errorf("BaseURL lost: %q", got.BaseURL)
		}
	})

	t.Run("default preserved when in selection", func(t *testing.T) {
		probe := config.ProviderEntry{
			Name: "deepseek", Default: "deepseek-v4-pro",
			BaseURL: "https://api.deepseek.com",
		}
		got := buildFamilyEntry(probe, []string{"deepseek-v4-flash", "deepseek-v4-pro"})
		if got.Default != "deepseek-v4-pro" {
			t.Errorf("Default = %q, want preserved", got.Default)
		}
	})

	t.Run("empty default filled from first selected", func(t *testing.T) {
		probe := config.ProviderEntry{Name: "x", BaseURL: "u"}
		got := buildFamilyEntry(probe, []string{"alpha", "beta"})
		if got.Default != "alpha" {
			t.Errorf("Default = %q, want alpha", got.Default)
		}
	})
}

// for unparseable URLs. The exact format isn't load-bearing — what matters
// calls with the same URL, and (c) never produces the bare "custom" /
// "anthropic" magic names that would collide with the wizard menu items.
func TestProviderSlug(t *testing.T) {
	cases := []struct {
		name, kind, url, want string
	}{
		{"standard host with port", "custom", "https://token.sensenova.cn/v1", "custom-token-sensenova-cn"},
		{"api subdomain", "custom", "https://api.openai.com/v1", "custom-api-openai-com"},
		{"www stripped", "custom", "https://www.example.com/v1", "custom-example-com"},
		{"port preserved", "custom", "http://localhost:11434/v1", "custom-localhost-11434"},
		{"anthropic kind", "anthropic", "https://api.anthropic.com", "anthropic-api-anthropic-com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerSlug(tc.kind, tc.url); got != tc.want {
				t.Errorf("providerSlug(%q, %q) = %q, want %q", tc.kind, tc.url, got, tc.want)
			}
		})
	}

	t.Run("stable across calls", func(t *testing.T) {
		a := providerSlug("custom", "https://token.sensenova.cn/v1")
		b := providerSlug("custom", "https://token.sensenova.cn/v1")
		if a != b {
			t.Errorf("not stable: %q vs %q", a, b)
		}
		if a == "custom" {
			t.Error("slug degenerated to bare magic name — collision risk")
		}
	})

	t.Run("sha1 fallback for unparseable URL", func(t *testing.T) {
		got := providerSlug("custom", "://not a url::://")
		if !strings.HasPrefix(got, "custom-") || got == "custom" {
			t.Errorf("fallback slug = %q, want custom-<hex>", got)
		}
		if len(got) != len("custom-")+8 {
			t.Errorf("fallback slug = %q, want 8 hex chars after prefix", got)
		}
	})

	t.Run("sha1 fallback for non-ascii host", func(t *testing.T) {
		got := providerSlug("custom", "https://예시.테스트/v1")
		if !strings.HasPrefix(got, "custom-") || got == "custom-" {
			t.Errorf("fallback slug = %q, want custom-<hex>", got)
		}
		if len(got) != len("custom-")+8 {
			t.Errorf("fallback slug = %q, want 8 hex chars after prefix", got)
		}
	})
}

func TestAPIKeyEnvFromProviderName(t *testing.T) {
	cases := []struct {
		name, providerName, want string
	}{
		{"custom host slug", "custom-token-sensenova-cn", "CUSTOM_TOKEN_SENSENOVA_CN_API_KEY"},
		{"localhost slug with port", "custom-localhost-11434", "CUSTOM_LOCALHOST_11434_API_KEY"},
		{"desktop-style custom name", "Local Gateway", "LOCAL_GATEWAY_API_KEY"},
		{"digit-leading provider name", "9router", "CUSTOM_9ROUTER_API_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiKeyEnvFromProviderName(tc.providerName); got != tc.want {
				t.Errorf("apiKeyEnvFromProviderName(%q) = %q, want %q", tc.providerName, got, tc.want)
			}
		})
	}

	t.Run("non-ascii provider names use desktop-compatible hash fallback", func(t *testing.T) {
		if got, want := apiKeyEnvFromProviderName("샹탕"), "CUSTOM_47740c73_API_KEY"; got != want {
			t.Errorf("apiKeyEnvFromProviderName(non-ascii) = %q, want %q", got, want)
		}
		if got := apiKeyEnvFromProviderName("퉁이첸원"); got == "CUSTOM_47740c73_API_KEY" || got == "CUSTOM_API_KEY" {
			t.Errorf("apiKeyEnvFromProviderName(second non-ascii) = %q, want distinct stable fallback", got)
		}
	})
}

func TestPromptCustomProviderManualDefaultsKeyEnvFromBaseURL(t *testing.T) {
	result, err := promptCustomProviderManualWith(
		bufio.NewScanner(strings.NewReader("sensenova-chat\n\n\n")),
		"https://token.sensenova.cn/v1",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("promptCustomProviderManualWith: %v", err)
	}
	entries := result.entries
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got, want := entries[0].APIKeyEnv, "CUSTOM_TOKEN_SENSENOVA_CN_API_KEY"; got != want {
		t.Errorf("APIKeyEnv = %q, want %q", got, want)
	}
}

func TestPromptCustomProviderManualPreservesExplicitKeyEnv(t *testing.T) {
	result, err := promptCustomProviderManualWith(
		bufio.NewScanner(strings.NewReader("manual-chat\n\n")),
		"https://token.sensenova.cn/v1",
		"CUSTOM_API_KEY",
		"",
	)
	if err != nil {
		t.Fatalf("promptCustomProviderManualWith: %v", err)
	}
	entries := result.entries
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := entries[0].APIKeyEnv; got != "CUSTOM_API_KEY" {
		t.Errorf("APIKeyEnv = %q, want explicit CUSTOM_API_KEY", got)
	}
}

func TestPromptAPIKeyEnvNameRejectsModelName(t *testing.T) {
	i18n.DetectLanguage("en")
	var out bytes.Buffer
	got := promptAPIKeyEnvName(
		bufio.NewScanner(strings.NewReader("grok-4.5\n\n")),
		&out,
		i18n.M.CustomPromptKeyEnv,
		"CUSTOM_API_YAIROUTER_COM_API_KEY",
	)
	if got != "CUSTOM_API_YAIROUTER_COM_API_KEY" {
		t.Fatalf("key env = %q, want generated default", got)
	}
	if text := out.String(); !strings.Contains(text, "not a valid API Key variable name") || !strings.Contains(text, "do not enter a model name") {
		t.Fatalf("validation guidance missing from prompt output: %q", text)
	}
}

func TestPromptCustomProviderManualAsksForModelBeforeCredentialName(t *testing.T) {
	result, err := promptCustomProviderManualWith(
		bufio.NewScanner(strings.NewReader("grok-4.5\ngrok-4.5\n\n\n")),
		"https://api.example.com/v1",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("promptCustomProviderManualWith: %v", err)
	}
	entries := result.entries
	if got := entries[0].Model; got != "grok-4.5" {
		t.Fatalf("model = %q, want grok-4.5", got)
	}
	if got := entries[0].APIKeyEnv; got != "CUSTOM_API_EXAMPLE_COM_API_KEY" {
		t.Fatalf("APIKeyEnv = %q, want generated default after invalid model-like input", got)
	}
}

func TestPromptCustomProviderStagesExplicitKeyEvenWhenProcessEnvMatches(t *testing.T) {
	const key = "CUSTOM_API_EXAMPLE_COM_API_KEY"
	t.Setenv(key, "same-secret")
	result, err := promptCustomProviderManualWith(
		bufio.NewScanner(strings.NewReader("grok-4.5\n")),
		"https://api.example.com/v1",
		key,
		"same-secret",
	)
	if err != nil {
		t.Fatalf("promptCustomProviderManualWith: %v", err)
	}
	if got := result.credentials[key]; got != "same-secret" {
		t.Fatalf("staged credential = %q, want explicitly entered value", got)
	}
	if got := os.Getenv(key); got != "same-secret" {
		t.Fatalf("prompt changed process environment to %q", got)
	}
	result, err = promptCustomProviderManualWith(
		bufio.NewScanner(strings.NewReader("grok-4.5\n")),
		"https://api.example.com/v1",
		key,
		"new-secret",
	)
	if err != nil {
		t.Fatalf("promptCustomProviderManualWith with replacement key: %v", err)
	}
	if got := result.credentials[key]; got != "new-secret" {
		t.Fatalf("replacement staged credential = %q", got)
	}
	if got := os.Getenv(key); got != "same-secret" {
		t.Fatalf("prompt leaked replacement credential into process environment: %q", got)
	}
}

func TestRepairInvalidProviderKeyEnvs(t *testing.T) {
	original := []config.ProviderEntry{
		{Name: "custom-relay-example-com", APIKeyEnv: "grok-4.5"},
		{Name: "valid", APIKeyEnv: "VALID_API_KEY"},
		{Name: "no-auth"},
	}
	got, repairs := repairInvalidProviderKeyEnvs(original)
	if len(repairs) != 1 {
		t.Fatalf("repairs = %+v, want one", repairs)
	}
	if got[0].APIKeyEnv != "CUSTOM_RELAY_EXAMPLE_COM_API_KEY" {
		t.Fatalf("repaired key env = %q", got[0].APIKeyEnv)
	}
	if repairs[0].old != "grok-4.5" || repairs[0].new != got[0].APIKeyEnv {
		t.Fatalf("repair detail = %+v", repairs[0])
	}
	if got[1].APIKeyEnv != "VALID_API_KEY" || got[2].APIKeyEnv != "" {
		t.Fatalf("valid/no-auth providers changed: %+v", got)
	}
	if original[0].APIKeyEnv != "grok-4.5" {
		t.Fatalf("repair mutated caller input: %+v", original[0])
	}
}

// TestFilterStaleCustomEntries covers the wizards auto-cleanup of legacy
// "custom" / "anthropic" magic-name entries that previous versions wrote
// into patty.toml. These collide with the wizards own menu items, so
// they're dropped from the providers list before grouping — but the caller
func TestFilterStaleCustomEntries(t *testing.T) {
	in := []config.ProviderEntry{
		{Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com"},
		{Name: "custom", Kind: "openai", BaseURL: "https://old.example/v1"},                // stale
		{Name: "anthropic", Kind: "anthropic", BaseURL: "https://old.example/v1/messages"}, // stale
		{Name: "mimo-tp", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1"},
	}
	kept, dropped := filterStaleCustomEntries(in)
	if len(kept) != 2 {
		t.Errorf("kept = %d entries, want 2: %+v", len(kept), kept)
	}
	if len(dropped) != 2 {
		t.Errorf("dropped = %d entries, want 2: %+v", len(dropped), dropped)
	}
	for _, k := range kept {
		if k.Name == "custom" || k.Name == "anthropic" {
			t.Errorf("magic name leaked through: %q", k.Name)
		}
	}

	t.Run("non-magic names with kind anthropic are kept", func(t *testing.T) {
		// An entry someone deliberately named "claude" (kind=anthropic) must
		// not be touched by the filter — only the bare "anthropic" magic name.
		in := []config.ProviderEntry{
			{Name: "claude", Kind: "anthropic", BaseURL: "https://api.anthropic.com"},
		}
		kept, dropped := filterStaleCustomEntries(in)
		if len(kept) != 1 || len(dropped) != 0 {
			t.Errorf("claude should be kept, got kept=%d dropped=%d", len(kept), len(dropped))
		}
	})

	t.Run("custom kind anthropic is kept", func(t *testing.T) {
		// Name="custom" with kind=anthropic is ambiguous — keep it.
		in := []config.ProviderEntry{
			{Name: "custom", Kind: "anthropic", BaseURL: "https://x"},
		}
		kept, dropped := filterStaleCustomEntries(in)
		if len(kept) != 1 || len(dropped) != 0 {
			t.Errorf("custom+anthropic should be kept (ambiguous), got kept=%d dropped=%d", len(kept), len(dropped))
		}
	})
}

func TestWithBuiltinFamiliesDoesNotAddMissingMimo(t *testing.T) {
	// The users case: a patty.toml that defines only deepseek providers.
	cfg := []config.ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com"},
		{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com"},
	}
	order, _, info := groupByFamily(withBuiltinFamilies(cfg))
	seen := map[string]bool{}
	for _, k := range order {
		seen[info[k].name] = true
	}
	if !seen["DeepSeek"] {
		t.Fatalf("wizard families = %v, want DeepSeek", order)
	}
	if !seen["Patty"] {
		t.Fatalf("wizard families = %v, want Patty", order)
	}
	if seen["MiMo (Xiaomi)"] {
		t.Fatalf("wizard families = %v, should not inject MiMo", order)
	}
	// A users customized deepseek must not be duplicated.
	if n := len(groupByFamilyKeys(withBuiltinFamilies(cfg), "deepseek")); n != 2 {
		t.Fatalf("deepseek members = %d, want the user's 2 (no injected duplicate)", n)
	}
}

func TestWithBuiltinFamiliesUsesPattyCodeStandard(t *testing.T) {
	providers := withBuiltinFamilies(nil)
	if len(providers) != 1 || providers[0].Name != "patty" || providers[0].Model != "patty-code-standard" {
		t.Fatalf("built-in providers = %+v, want Patty patty-code-standard", providers)
	}
}

// with a single model). Re-running `patcode setup` must still surface the
func TestWithBuiltinFamiliesPreservesUserDeepSeekAndAddsPatty(t *testing.T) {
	cfg := []config.ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", Models: []string{"deepseek-v4-flash"}, APIKeyEnv: "DEEPSEEK_API_KEY"},
	}
	got := withBuiltinFamilies(cfg)

	var foundPatty bool
	for _, p := range got {
		if p.Name == "patty" {
			foundPatty = true
		}
		if p.Name == "deepseek-pro" {
			t.Fatalf("withBuiltinFamilies unexpectedly restored a non-stock DeepSeek sibling: %v", namesOf(got))
		}
	}
	if !foundPatty {
		t.Fatalf("withBuiltinFamilies(%+v) = %v, want Patty stock entry", cfg, namesOf(got))
	}
}

func namesOf(ps []config.ProviderEntry) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

func groupByFamilyKeys(ps []config.ProviderEntry, key string) []int {
	_, members, _ := groupByFamily(ps)
	return members[key]
}

func TestWriteDefaultConfigOmitsLegacyInternalMCPSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patty.toml")
	if rc := writeDefaultConfig(path); rc != 0 {
		t.Fatalf("writeDefaultConfig rc = %d", rc)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"[codegraph]", "[builtin_mcp]", "[builtin_mcp_updates]"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("default config should omit %s:\n%s", forbidden, text)
		}
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func captureCLIOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, fn)
	})
	return stdout, stderr
}

func TestProvidersWithMissingKeysOnlyReferenced(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv("DARI_HARNESS_KEY", "")
	t.Setenv("MIMO_API_KEY", "")
	cfg := config.Default()

	got := providersWithMissingKeys(cfg)
	envs := map[string]bool{}
	for _, p := range got {
		envs[p.APIKeyEnv] = true
	}
	if !envs["DARI_HARNESS_KEY"] {
		t.Errorf("the default model's missing key must be prompted, got %v", got)
	}
	if envs["MIMO_API_KEY"] {
		t.Errorf("unreferenced preset keys must not be prompted, got %v", got)
	}
}

func TestProvidersWithMissingKeysIncludesPlannerModel(t *testing.T) {
	isolateCLIConfigHome(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv("MIMO_API_KEY", "")
	cfg := config.Default()
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{Name: "mimo-pro", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5-pro", APIKeyEnv: "MIMO_API_KEY"})
	cfg.Agent.PlannerModel = "mimo-pro"

	got := providersWithMissingKeys(cfg)
	if len(got) != 1 || got[0].APIKeyEnv != "MIMO_API_KEY" {
		t.Errorf("planner model's missing key must be prompted, got %+v", got)
	}
}

func TestParseRuntimeProfile(t *testing.T) {
	for input, want := range map[string]string{
		"":         boot.TokenModeFull,
		"balanced": boot.TokenModeFull,
		"full":     boot.TokenModeFull,
		"economy":  boot.TokenModeEconomy,
		"delivery": boot.TokenModeDelivery,
	} {
		got, err := parseRuntimeProfile(input)
		if err != nil || got != want {
			t.Errorf("parseRuntimeProfile(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := parseRuntimeProfile("fast"); err == nil {
		t.Fatal("unknown profile should fail")
	}
}
