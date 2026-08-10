package cli

import (
	"fmt"
	"strings"

	"patty/internal/config"
)

func (m *chatTUI) runReasoningLanguageCommand(input string) {
	args := tokenizeArgs(input)
	if len(args) < 2 {
		cfg, err := config.Load()
		if err != nil {
			m.notice("reasoning-language: " + err.Error())
			return
		}
		m.notice(fmt.Sprintf("reasoning-language: %s (usage: /reasoning-language auto|ko-KR|en)", cliReasoningLanguageMode(cfg.ReasoningLanguage())))
		return
	}
	if len(args) > 2 {
		m.notice("usage: /reasoning-language auto|ko-KR|en")
		return
	}
	if m.ctrl != nil && m.ctrl.Running() {
		m.notice("finish or cancel the current turn before changing reasoning-language")
		return
	}
	mode, err := parseCLIReasoningLanguage(args[1])
	if err != nil {
		m.notice("reasoning-language: " + err.Error())
		return
	}

	path := config.UserConfigPath()
	if path == "" {
		m.notice("reasoning-language: cannot resolve config path")
		return
	}
	mode, err = func() (string, error) {
		unlock := config.LockUserConfigEdits()
		defer unlock()
		edit := config.LoadForEdit(path)
		if err := edit.SetReasoningLanguage(mode); err != nil {
			return "", err
		}
		if err := edit.SaveTo(path); err != nil {
			return "", err
		}
		return edit.ReasoningLanguage(), nil
	}()
	if err != nil {
		m.notice("reasoning-language: " + err.Error())
		return
	}

	if m.ctrl != nil {
		m.ctrl.SetReasoningLanguage(mode)
	}
	m.notice(fmt.Sprintf("reasoning-language set to %s", mode))
}

func parseCLIReasoningLanguage(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return "auto", nil
	case "ko-kr", "ko":
		return "ko-KR", nil
	case "en":
		return "en", nil
	default:
		return "", fmt.Errorf("reasoning_language %q: must be auto|ko-KR|en", mode)
	}
}

func cliReasoningLanguageMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ko-kr", "ko":
		return "ko-KR"
	case "en":
		return "en"
	default:
		return "auto"
	}
}
