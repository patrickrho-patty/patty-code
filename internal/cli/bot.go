package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"patty/internal/bot"
	"patty/internal/botruntime"
	"patty/internal/config"
)

func botCommand(args []string, version string) int {
	if len(args) < 1 {
		botUsage()
		return 2
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "start":
		return botStart(rest, version)
	case "doctor":
		return botDoctor(rest)
	case "pairing":
		return botPairing(rest)
	case "help", "--help", "-h":
		botUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown bot subcommand %q\n\n", sub)
		botUsage()
		return 2
	}
}

func botStart(args []string, version string) int {
	fs := flag.NewFlagSet("bot start", flag.ContinueOnError)
	channels := fs.String("channels", "", "활성화할 채널 (연결 provider 이름, 쉼표 구분)")
	dir := fs.String("dir", "", "작업 디렉터리")
	model := fs.String("model", "", "모델 이름 (비어 있으면 default_model 사용)")

	if code, ok := parseCommandFlags(fs, args); !ok {
		return code
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := loadBotCommandConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	if !cfg.Bot.Enabled {
		fmt.Fprintln(os.Stderr, "error: bot is not enabled in config — set [bot] enabled = true")
		return 1
	}
	if !botruntime.BotConfigHasAccessControl(cfg.Bot) {
		fmt.Fprintln(os.Stderr, "error: bot requires explicit access control; set per-connection access, enable pairing, configure [bot.allowlist], or set allow_all = true intentionally")
		return 1
	}

	workspaceRoot := *dir
	if workspaceRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceRoot = wd
		}
	}

	requestedChannels := splitBotChannels(*channels)
	enabledPlatforms, unknownChannels := botruntime.EnabledPlatforms(cfg, requestedChannels)
	for _, ch := range unknownChannels {
		fmt.Fprintf(os.Stderr, "warning: unknown channel %q\n", ch)
	}
	if !botruntime.HasEnabledPlatform(enabledPlatforms) {
		fmt.Fprintln(os.Stderr, "error: no bot channels enabled — enable at least one in config")
		return 1
	}

	modelName := botruntime.ModelName(cfg, *model)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	rememberInboundRemote := botruntime.NewRemoteRememberer(logger)

	gwCfg := bot.GatewayConfig{
		Model:              modelName,
		ToolApprovalMode:   cfg.Bot.ToolApprovalMode,
		MaxSteps:           cfg.Bot.MaxSteps,
		QueueMode:          cfg.Bot.QueueMode,
		QueueCap:           cfg.Bot.QueueCap,
		QueueDrop:          cfg.Bot.QueueDrop,
		PairingEnabled:     cfg.Bot.Pairing.Enabled,
		PairingTTL:         time.Duration(cfg.Bot.Pairing.RequestTTLMinutes) * time.Minute,
		PairingMaxPending:  cfg.Bot.Pairing.MaxPendingPerPlatform,
		IgnoreSelfMessages: cfg.Bot.IgnoreSelfMessages,
		SelfUserIDs: map[bot.Platform][]string{
			bot.Platform("desktop"): cfg.Bot.SelfUserIDs.Desktop,
		},
		ControlEnabled:     cfg.Bot.Control.Enabled,
		ControlAddr:        cfg.Bot.Control.Addr,
		ControlToken:       os.Getenv(strings.TrimSpace(cfg.Bot.Control.TokenEnv)),
		WorkspaceRoot:      workspaceRoot,
		Channels:           botruntime.ChannelConfigs(cfg.Bot.Connections, *model == "", *dir == ""),
		ConnectionChannels: botruntime.ConnectionChannelConfigs(cfg.Bot.Connections, *model == "", *dir == ""),
		Routes:             botruntime.RouteConfigs(cfg.Bot.Routes, *model == "", *dir == ""),
		ConnectionAccess:   botruntime.ConnectionAccessConfigs(cfg),
		Enabled:            enabledPlatforms,
		Allowlist: bot.AllowlistConfig{
			Enabled:  cfg.Bot.Allowlist.Enabled,
			AllowAll: cfg.Bot.Allowlist.AllowAll,
			Users: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Users,
			},
			Approvers: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Approvers,
			},
			Admins: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Admins,
			},
			Groups: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Groups,
			},
		},
		Debounce:       time.Duration(cfg.Bot.DebounceMs) * time.Millisecond,
		OnInbound:      rememberInboundRemote,
		OnSessionReady: botruntime.NewSessionRemembererWithWorkspace(logger, workspaceRoot),
	}

	gw := bot.NewGatewayWithAdapterBindings(gwCfg, botruntime.AdapterBindings(cfg, enabledPlatforms, logger), logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		cancel()
	}()

	fmt.Fprintf(os.Stderr, "patcode bot starting (model: %s, channels: %s)...\n", modelName, *channels)
	fmt.Fprintf(os.Stderr, "version: %s\n", version)

	if err := gw.Start(ctx); err != nil {
		gw.Stop()
		fmt.Fprintf(os.Stderr, "error: start gateway: %v\n", err)
		return 1
	}
	defer gw.Stop()

	<-ctx.Done()
	return 0
}

func splitBotChannels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func botDoctor(args []string) int {
	fs := flag.NewFlagSet("bot doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 형식으로 출력")
	deep := fs.Bool("deep", false, "더 자세한 로컬 진단 실행")

	if code, ok := parseCommandFlags(fs, args); !ok {
		return code
	}

	cfg, err := loadBotCommandConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	bc := cfg.Bot

	type checkResult struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
	}

	var results []checkResult

	addCheck := func(name, status, detail string) {
		results = append(results, checkResult{Name: name, Status: status, Detail: detail})
	}

	if bc.Enabled {
		addCheck("bot.enabled", "ok", "")
	} else {
		addCheck("bot.enabled", "disabled", "bot is not enabled in config")
	}
	if *deep {
		if path := config.UserConfigPath(); path != "" {
			if _, err := os.Stat(path); err == nil {
				addCheck("bot.config.user", "ok", path)
			} else {
				addCheck("bot.config.user", "missing", path)
			}
		}
		if dir := config.SessionDir(); dir != "" {
			addCheck("bot.sessions.dir", "ok", dir)
		}
	}
	queueMode := bot.NormalizeQueueMode(bc.QueueMode)
	queueCap := bc.QueueCap
	if queueCap <= 0 {
		queueCap = bot.DefaultQueueCap
	}
	addCheck("bot.queue", "ok", fmt.Sprintf("mode=%s cap=%d drop=%s", queueMode, queueCap, bot.NormalizeQueueDrop(bc.QueueDrop)))
	if bc.Pairing.Enabled {
		addCheck("bot.pairing", "enabled", fmt.Sprintf("ttl=%dm max_pending=%d", bc.Pairing.RequestTTLMinutes, bc.Pairing.MaxPendingPerPlatform))
	} else {
		addCheck("bot.pairing", "disabled", "")
	}
	if *deep {
		reqs, err := bot.ListPairingRequests()
		if err != nil {
			addCheck("bot.pairing.pending", "error", err.Error())
		} else {
			addCheck("bot.pairing.pending", "ok", fmt.Sprintf("%d pending", len(reqs)))
		}
		if path := bot.PairingStorePath(); path != "" {
			if info, err := os.Stat(path); err == nil {
				addCheck("bot.pairing.store", "ok", fmt.Sprintf("%s mode=%s", path, info.Mode().Perm()))
			} else {
				addCheck("bot.pairing.store", "missing", path)
			}
		}
	}
	if *deep {
		selfStatus := "disabled"
		if bc.IgnoreSelfMessages {
			selfStatus = "enabled"
		}
		addCheck("bot.self_protection", selfStatus,
			fmt.Sprintf("self_ids=%d", len(bc.SelfUserIDs.Desktop)))
		controlStatus := "disabled"
		controlDetail := ""
		if bc.Control.Enabled {
			controlStatus = "enabled"
			tokenStatus := "missing_token"
			if strings.TrimSpace(bc.Control.TokenEnv) != "" && os.Getenv(strings.TrimSpace(bc.Control.TokenEnv)) != "" {
				tokenStatus = "token_set"
			}
			addr := strings.TrimSpace(bc.Control.Addr)
			if addr == "" {
				addr = "127.0.0.1:37913"
			}
			controlDetail = fmt.Sprintf("addr=%s token_env=%s %s", addr, bc.Control.TokenEnv, tokenStatus)
		}
		addCheck("bot.control", controlStatus, controlDetail)
		addCheck("bot.routes", "ok", fmt.Sprintf("%d routes", len(bc.Routes)))
	}

	enabledConnections := 0
	for _, conn := range bc.Connections {
		if conn.Enabled {
			enabledConnections++
		}
	}
	addCheck("bot.connections", "ok", fmt.Sprintf("enabled=%d total=%d", enabledConnections, len(bc.Connections)))
	for _, conn := range bc.Connections {
		id := strings.TrimSpace(conn.ID)
		if id == "" {
			id = strings.TrimSpace(conn.Provider)
		}
		status := "ok"
		if !conn.Enabled {
			status = "disabled"
		}
		addCheck("bot.connection."+id+".session_mappings", status,
			fmt.Sprintf("provider=%s mappings=%d", conn.Provider, len(conn.SessionMappings)))
	}

	if bc.Allowlist.AllowAll {
		addCheck("bot.allowlist", "open", "allow_all=true — every reachable user can trigger local tools")
	} else if bc.Allowlist.Enabled {
		addCheck("bot.allowlist", "enabled",
			fmt.Sprintf("users=%d approvers=%d admins=%d",
				len(bc.Allowlist.Users),
				len(bc.Allowlist.Approvers),
				len(bc.Allowlist.Admins)))
	} else {
		addCheck("bot.allowlist", "missing", "bot start will refuse without allowlist or allow_all=true")
	}
	if *deep {
		addCheck("bot.roles", "ok",
			fmt.Sprintf("approvers=%d admins=%d",
				len(bc.Allowlist.Approvers),
				len(bc.Allowlist.Admins)))
	}

	if *jsonOut {
		fmt.Println("[")
		for i, r := range results {
			comma := ","
			if i == len(results)-1 {
				comma = ""
			}
			fmt.Printf("  {\"name\":%q,\"status\":%q,\"detail\":%q}%s\n", r.Name, r.Status, r.Detail, comma)
		}
		fmt.Println("]")
	} else {
		for _, r := range results {
			marker := "✓"
			if r.Status == "missing" || r.Status == "disabled" {
				marker = "✗"
			}
			fmt.Printf("  %s %s: %s", marker, r.Name, r.Status)
			if r.Detail != "" {
				fmt.Printf(" — %s", r.Detail)
			}
			fmt.Println()
		}
	}

	return 0
}

func botPairing(args []string) int {
	if len(args) < 1 {
		botPairingUsage()
		return 2
	}
	switch args[0] {
	case "list":
		reqs, err := bot.ListPairingRequests()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: list pairing requests: %v\n", err)
			return 1
		}
		if len(reqs) == 0 {
			fmt.Println("No pending bot pairing requests.")
			return 0
		}
		for _, req := range reqs {
			fmt.Printf("%s\t%s\t%s\tuser=%s\tchat=%s\texpires=%s\n",
				req.Code,
				req.Platform,
				req.ChatType,
				req.UserID,
				req.ChatID,
				req.ExpiresAt.Local().Format("2006-01-02 15:04"),
			)
		}
		return 0
	case "approve":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "error: pairing approve requires a code")
			return 2
		}
		req, err := bot.ApprovePairingCode(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: approve pairing: %v\n", err)
			return 1
		}
		fmt.Printf("Approved %s user %s for %s.\n", req.Platform, req.UserID, req.ChatID)
		return 0
	case "reject", "deny":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "error: pairing reject requires a code")
			return 2
		}
		req, err := bot.RejectPairingCode(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reject pairing: %v\n", err)
			return 1
		}
		fmt.Printf("Rejected %s user %s for %s.\n", req.Platform, req.UserID, req.ChatID)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown bot pairing subcommand %q\n\n", args[0])
		botPairingUsage()
		return 2
	}
}

func botPairingUsage() {
	fmt.Print(`patcode bot pairing — approve pending bot DM pairings

Usage:
  patcode bot pairing list
  patcode bot pairing approve CODE
  patcode bot pairing reject CODE
`)
}

func loadBotCommandConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	userPath := config.UserConfigPath()
	if strings.TrimSpace(userPath) == "" {
		return cfg, nil
	}
	if _, err := os.Stat(userPath); err != nil {
		return cfg, nil
	}
	userCfg := config.LoadForEdit(userPath)
	if botConfigIsUserOwned(userCfg.Bot) {
		cfg.Bot = userCfg.Bot
	}
	return cfg, nil
}

func botConfigIsUserOwned(bc config.BotConfig) bool {
	if bc.Enabled || len(bc.Connections) > 0 {
		return true
	}
	if bc.Allowlist.AllowAll || botruntime.AllowlistUserCount(bc.Allowlist) > 0 {
		return true
	}
	for _, conn := range bc.Connections {
		if botruntime.BotAccessActive(conn.Access) {
			return true
		}
	}
	return len(bc.Allowlist.Groups)+len(bc.Allowlist.Approvers)+len(bc.Allowlist.Admins) > 0
}

func botUsage() {
	fmt.Print(`patcode bot — multi-channel IM bot gateway

Usage:
  patcode bot start   [--channels NAME,...] [--dir PATH] [--model NAME]
  patcode bot doctor  [--json] [--deep]
  patcode bot pairing list|approve|reject

Subcommands:
  start          bot 게이트웨이 시작
  doctor         bot 구성과 연결성 진단
  pairing        IM 개인 채팅 페어링 조회/승인

Examples:
  patcode bot start --channels my-channel
  patcode bot start --dir /path/to/project --model deepseek-pro
  patcode bot doctor --json

Configuration:
  Edit patty.toml:
    [bot]           enabled / model / max_steps
    [bot]           queue_mode / queue_cap / queue_drop
    [bot.pairing]   enabled / request_ttl_minutes / max_pending_per_platform
    [bot.allowlist]  enabled / users / approvers / admins / groups

  All secrets are read from environment variables; never put keys in config files.
`)
}
