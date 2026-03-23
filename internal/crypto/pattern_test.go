package crypto_test

import (
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestWrapUnwrap(t *testing.T) {
	raw := "SGVsbG8gV29ybGQ="
	wrapped := crypto.WrapENC(raw)
	if wrapped != "ENC[age:SGVsbG8gV29ybGQ=]" {
		t.Errorf("unexpected wrap: %s", wrapped)
	}
	unwrapped, err := crypto.UnwrapENC(wrapped)
	if err != nil {
		t.Fatalf("unwrap failed: %v", err)
	}
	if unwrapped != raw {
		t.Errorf("expected %s, got %s", raw, unwrapped)
	}
}

func TestUnwrapInvalid(t *testing.T) {
	cases := []string{"not-encrypted", "ENC[wrong:data]", "ENC[age:]", "ENC[age:data"}
	for _, c := range cases {
		_, err := crypto.UnwrapENC(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestIsEncrypted(t *testing.T) {
	if !crypto.IsEncrypted("ENC[age:abc123]") {
		t.Error("should detect ENC pattern")
	}
	if crypto.IsEncrypted("plaintext") {
		t.Error("should not detect plaintext")
	}
}

func TestFindAllENCPatterns(t *testing.T) {
	input := "Authorization: Bearer ENC[age:token1]\nX-Api-Key: ENC[age:token2]\nContent-Type: application/json"
	matches := crypto.FindAllENC(input)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0] != "ENC[age:token1]" {
		t.Errorf("unexpected match 0: %s", matches[0])
	}
}

func TestReplaceAllENC(t *testing.T) {
	input := "key=ENC[age:aaa] other=ENC[age:bbb]"
	mapping := map[string]string{"ENC[age:aaa]": "secret1", "ENC[age:bbb]": "secret2"}
	result := crypto.ReplaceAllENC(input, mapping)
	if !strings.Contains(result, "key=secret1") || !strings.Contains(result, "other=secret2") {
		t.Errorf("replacement failed: %s", result)
	}
}
