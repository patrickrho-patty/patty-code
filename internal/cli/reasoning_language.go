package cli

import (
	"fmt"
	"strings"
)

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
