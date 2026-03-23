package crypto

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	encPrefix = "ENC[age:"
	encSuffix = "]"
)

var encPattern = regexp.MustCompile(`ENC\[age:[A-Za-z0-9+/=]+\]`)

// WrapENC wraps a base64-encoded ciphertext in the ENC[age:...] pattern.
func WrapENC(base64Ciphertext string) string {
	return encPrefix + base64Ciphertext + encSuffix
}

// UnwrapENC extracts the base64-encoded ciphertext from an ENC[age:...] pattern.
func UnwrapENC(wrapped string) (string, error) {
	if !strings.HasPrefix(wrapped, encPrefix) {
		return "", fmt.Errorf("missing ENC[age: prefix")
	}
	if !strings.HasSuffix(wrapped, encSuffix) {
		return "", fmt.Errorf("missing ] suffix")
	}
	inner := wrapped[len(encPrefix) : len(wrapped)-len(encSuffix)]
	if inner == "" {
		return "", fmt.Errorf("empty ciphertext")
	}
	return inner, nil
}

// IsEncrypted returns true if the string contains an ENC[age:...] pattern.
func IsEncrypted(s string) bool {
	return encPattern.MatchString(s)
}

// FindAllENC returns all ENC[age:...] patterns found in the string.
func FindAllENC(s string) []string {
	return encPattern.FindAllString(s, -1)
}

// ReplaceAllENC replaces all ENC[age:...] patterns in the string using
// the provided mapping from encrypted tokens to plaintext values.
func ReplaceAllENC(s string, mapping map[string]string) string {
	result := s
	for enc, plain := range mapping {
		result = strings.ReplaceAll(result, enc, plain)
	}
	return result
}
