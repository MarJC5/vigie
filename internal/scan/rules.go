package scan

import (
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
)

// The thresholds of the rules: Vigie's choices, named in the interface's help texts.
const (
	expiryUrgent    = 7 * 24 * time.Hour  // a certificate or a domain expiring this soon is urgent
	expiryAttention = 30 * 24 * time.Hour // and needs attention this soon
	cronLate        = time.Hour           // a WP-Cron event overdue by more is reported
	autoloadLimit   = 800_000             // bytes, WordPress's own Site Health limit
	filesEvery      = 24 * time.Hour      // the files are compared at most this often
)

// vulnerabilityLevel: urgent when scored high or critical, known to be exploited, or fixed in no
// version; any other record needs attention, also one without a score.
func vulnerabilityLevel(v lookup.Vulnerability) model.FindingLevel {
	if v.Severity == "high" || v.Severity == "critical" || v.KEV || v.Unfixed {
		return model.LevelUrgent
	}
	return model.LevelAttention
}

// closedLevel: a plugin closed on wordpress.org for a security issue is urgent; for another
// reason, it needs attention.
func closedLevel(reason string) model.FindingLevel {
	if reason == "security-issue" {
		return model.LevelUrgent
	}
	return model.LevelAttention
}

// fileLevel: a WordPress file changed, missing or added is urgent; a plugin's needs attention,
// since plugins are sometimes patched on purpose.
func fileLevel(plugin string) model.FindingLevel {
	if plugin == "" {
		return model.LevelUrgent
	}
	return model.LevelAttention
}

// expiryLevel: a certificate or a domain expiring within 7 days is urgent, within 30 days needs
// attention; later, it is only shown.
func expiryLevel(left time.Duration) model.FindingLevel {
	switch {
	case left <= expiryUrgent:
		return model.LevelUrgent
	case left <= expiryAttention:
		return model.LevelAttention
	}
	return model.LevelInfo
}

// twoFactorLevel: an administrator without two-factor needs attention; with it, or when it cannot
// be read, they are only listed.
func twoFactorLevel(state string) model.FindingLevel {
	if state == twoFactorOff {
		return model.LevelAttention
	}
	return model.LevelInfo
}

// The two-factor states of an administrator.
const (
	twoFactorOn      = "on"
	twoFactorOff     = "off"
	twoFactorUnknown = "unknown" // Wordfence is active and its table cannot be read
)
