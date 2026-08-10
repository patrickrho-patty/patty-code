package permission

import "patty/internal/shellsafe"

func normalizeBashSafeRedirectsForMatch(subject string) (string, bool) {
	return shellsafe.NormalizeBashSafeRedirectsForMatch(subject)
}
