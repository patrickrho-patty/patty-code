package main

import (
	"testing"

	"patty/internal/config"
	"patty/internal/control"
)

func TestDefaultTopicTitleLocalizesAtAPIBoundary(t *testing.T) {
	app := &App{}
	for _, tt := range []struct {
		locale string
		want   string
	}{
		{locale: "en-US", want: defaultTopicTitleEn},
		{locale: "ko-KR", want: defaultTopicTitleEn},
		{locale: "en-US", want: defaultTopicTitleEn},
	} {
		app.setDesktopLocale(tt.locale)
		if got := app.localizedTopicTitle(defaultTopicTitle, topicTitleSourceAuto); got != tt.want {
			t.Fatalf("locale %q title = %q, want %q", tt.locale, got, tt.want)
		}
	}
}

func TestManualDefaultTopicTitleIsNotLocalized(t *testing.T) {
	app := &App{}
	for _, tt := range []struct {
		locale string
		title  string
	}{
		{locale: "ko-KR", title: defaultTopicTitleEn},
		{locale: "en-US", title: defaultTopicTitle},
	} {
		app.setDesktopLocale(tt.locale)
		if got := app.localizedTopicTitle(tt.title, topicTitleSourceManual); got != tt.title {
			t.Fatalf("locale %q manual title = %q, want %q", tt.locale, got, tt.title)
		}
	}
}

func TestCreateTopicPreservesManualDefaultTitle(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := &App{}
	app.projectTreeChangedHook = func() {}
	app.setDesktopLocale("ko-KR")

	manual, err := app.CreateTopic("global", "", defaultTopicTitleEn)
	if err != nil {
		t.Fatalf("CreateTopic manual: %v", err)
	}
	if manual.Title != defaultTopicTitleEn {
		t.Fatalf("manual title = %q, want %q", manual.Title, defaultTopicTitleEn)
	}
	if got := loadTopicTitleSource("", manual.ID); got != topicTitleSourceManual {
		t.Fatalf("manual title source = %q, want %q", got, topicTitleSourceManual)
	}
	tree := app.ListProjectTree()
	if len(tree) == 0 || len(tree[0].Children) == 0 || tree[0].Children[0].Label != defaultTopicTitleEn {
		t.Fatalf("manual project-tree title was localized: %+v", tree)
	}

	automatic, err := app.CreateTopic("global", "", "")
	if err != nil {
		t.Fatalf("CreateTopic automatic: %v", err)
	}
	if automatic.Title != defaultTopicTitleEn {
		t.Fatalf("automatic title = %q, want localized %q", automatic.Title, defaultTopicTitleEn)
	}
	if err := app.RenameTopic(automatic.ID, defaultTopicTitleEn); err != nil {
		t.Fatalf("RenameTopic manual default: %v", err)
	}
	if got := loadTopicTitleSource("", automatic.ID); got != topicTitleSourceManual {
		t.Fatalf("renamed title source = %q, want %q", got, topicTitleSourceManual)
	}
	tree = app.ListProjectTree()
	if len(tree) == 0 || len(tree[0].Children) == 0 || tree[0].Children[0].Label != defaultTopicTitleEn {
		t.Fatalf("renamed project-tree title was localized: %+v", tree)
	}
}

func TestDefaultTopicTitleVariantsRemainPersistenceSentinels(t *testing.T) {
	for _, title := range []string{defaultTopicTitle, defaultTopicTitleEn} {
		if !isDefaultTopicTitle(title) {
			t.Fatalf("title %q was not recognized as a default sentinel", title)
		}
	}
	if got := topicTitleFromText(defaultTopicTitleEn); got != "" {
		t.Fatalf("English default title became a generated title: %q", got)
	}
}

func TestForkTopicTitleUsesDesktopLocale(t *testing.T) {
	app := &App{}
	app.setDesktopLocale("en")
	if got := app.forkTopicTitle("New session"); got != "Forked session" {
		t.Fatalf("English fork title = %q", got)
	}
	app.setDesktopLocale("en-US")
	if got := app.forkTopicTitle(defaultTopicTitle); got != "Forked session" {
		t.Fatalf("desktop fork title = %q", got)
	}
	app.desktopLocale.Store(desktopLocaleUnknown)
	if got := app.forkTopicTitle(""); got != "분기 세션" {
		t.Fatalf("legacy fallback fork title = %q", got)
	}
}

func TestSetTrayLocaleAutoCurrencyDoesNotRefreshForAnyLocale(t *testing.T) {
	isolateDesktopUserDirs(t)
	cfg := config.Default()
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save auto config: %v", err)
	}

	app := NewApp()
	app.projectTreeChangedHook = func() {}
	activeCtrl := control.New(control.Options{Label: "active"})
	inactiveCtrl := control.New(control.Options{Label: "inactive"})
	t.Cleanup(activeCtrl.Close)
	t.Cleanup(inactiveCtrl.Close)
	app.tabs = map[string]*WorkspaceTab{
		"active":   {ID: "active", Ctrl: activeCtrl},
		"inactive": {ID: "inactive", Ctrl: inactiveCtrl},
	}
	app.activeTabID = "active"
	app.setDesktopLocale("en")

	if err := app.SetTrayLocale("ko-KR"); err != nil {
		t.Fatalf("SetTrayLocale: %v", err)
	}
	for _, tabID := range []string{"active", "inactive"} {
		if app.deferredRebuildPending(tabID) {
			t.Fatalf("currency refresh for %q was scheduled despite an English-only auto currency", tabID)
		}
	}
}

func TestSetTrayLocaleKeepsExplicitCurrencyRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)
	cfg := config.Default()
	cfg.Desktop.Currency = "USD"
	cfg.ApplyDeepSeekOfficialDefaultPricing()
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save explicit currency config: %v", err)
	}

	app := NewApp()
	app.projectTreeChangedHook = func() {}
	ctrl := control.New(control.Options{Label: "active"})
	t.Cleanup(ctrl.Close)
	app.tabs = map[string]*WorkspaceTab{"active": {ID: "active", Ctrl: ctrl}}
	app.activeTabID = "active"
	app.setDesktopLocale("en")

	if err := app.SetTrayLocale("ko-KR"); err != nil {
		t.Fatalf("SetTrayLocale: %v", err)
	}
	if app.deferredRebuildPending("active") {
		t.Fatal("explicit USD currency scheduled an automatic locale refresh")
	}
}

func TestDesktopEffectivePricingCurrencyUsesLocaleOnlyForAuto(t *testing.T) {
	app := NewApp()
	app.setDesktopLocale("ko-KR")

	cfg := config.Default()
	if got := app.desktopEffectivePricingCurrency(cfg); got != "USD" {
		t.Fatalf("auto pricing currency = %q, want USD", got)
	}

	cfg.Desktop.Language = "en"
	cfg.ApplyDeepSeekOfficialDefaultPricing()
	if got := app.desktopEffectivePricingCurrency(cfg); got != "USD" {
		t.Fatalf("explicit desktop language pricing currency = %q, want USD", got)
	}

	cfg.Desktop.Language = ""
	cfg.Language = "en"
	cfg.ApplyDeepSeekOfficialDefaultPricing()
	if got := app.desktopEffectivePricingCurrency(cfg); got != "USD" {
		t.Fatalf("explicit CLI language pricing currency = %q, want USD", got)
	}

	cfg.Language = ""
	cfg.Desktop.Currency = "USD"
	cfg.ApplyDeepSeekOfficialDefaultPricing()
	if got := app.desktopEffectivePricingCurrency(cfg); got != "USD" {
		t.Fatalf("explicit currency pricing currency = %q, want USD", got)
	}
}
