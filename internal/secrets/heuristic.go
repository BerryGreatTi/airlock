package secrets

import "strings"

var secretKeywords = []string{"token", "key", "secret", "password", "credential", "auth"}

var secretPrefixes = []string{
	"sk-", "sk_live_", "sk_test_", "pk_live_", "pk_test_",
	"xoxb-", "xoxp-", "xoxa-", "xoxr-",
	"ghp_", "gho_", "ghs_", "ghu_",
	"glpat-",
	"AKIA",
	"eyJ",
	"whsec_",
	"rk_live_", "rk_test_",
}

func IsSecret(key, value string) bool {
	if isExcluded(value) {
		return false
	}
	return keyMatches(key) || valueMatches(value)
}

func isExcluded(value string) bool {
	if len(value) < 8 {
		return true
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return true
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	return false
}

func keyMatches(key string) bool {
	lower := strings.ToLower(key)
	for _, kw := range secretKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func valueMatches(value string) bool {
	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
