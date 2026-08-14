// Command patcode is a config- and plugin-driven coding agent CLI.
package main

import (
	"os"
	"runtime/debug"

	"patty/internal/cli"
	"patty/internal/config"
	"patty/internal/crashreport"

	// Blank imports wire compile-time built-ins into their registries.
	//
	// PAPER-only policy (PRD v2 §0.2): the official Harness must not contain
	// generic OpenAI/Anthropic/Responses providers for Patty service inference.
	// Only the PAPER provider is registered; the openai/anthropic/responses
	// packages are still in the source tree for PATTY_ALLOW_GENERIC=1 dev mode
	// but are NOT imported here, so they cannot be used in official builds.
	_ "patty/internal/provider/paper"
	_ "patty/internal/tool/builtin"
)

// Build identity injected via -ldflags (see Makefile). version remains the
// single-line contract for `patcode --version`; gitCommit/buildTimeUTC feed
// `patcode version --verbose` / `--json` without embedding config paths.
var (
	version      = "dev"
	gitCommit    = ""
	buildTimeUTC = ""
)

// runCLI is the CLI entry; tests may stub it. Production routes through
// RunWithBuildInfo so ldflags metadata is available to version --verbose/--json.
var runCLI = func(args []string, buildVersion string) int {
	return cli.RunWithBuildInfo(args, cli.BuildInfo{
		Version:      buildVersion,
		GitCommit:    gitCommit,
		BuildTimeUTC: buildTimeUTC,
	})
}

func main() {
	os.Exit(runWithCrashCapture(os.Args[1:], version))
}

func runWithCrashCapture(args []string, buildVersion string) (exitCode int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = crashreport.CapturePanic(config.PattyHomeDir(), buildVersion, recovered, debug.Stack())
			panic(recovered)
		}
	}()
	return runCLI(args, buildVersion)
}
