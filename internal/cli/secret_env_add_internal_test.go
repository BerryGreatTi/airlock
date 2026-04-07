package cli

import (
	"strings"
	"testing"
)

// Whitebox tests for readSecretValue. The function is unexported so the
// blackbox test file (`package cli_test`) cannot reach it; these tests
// live in `package cli` to exercise all four value-source branches.

func TestReadSecretValueFromValueFlag(t *testing.T) {
	got, err := readSecretValue(
		"GITHUB_TOKEN",
		"ghp_literal", true, // --value set
		false,                 // --stdin unset
		strings.NewReader(""), // stdin ignored
	)
	if err != nil {
		t.Fatalf("readSecretValue: %v", err)
	}
	if got != "ghp_literal" {
		t.Errorf("got %q, want ghp_literal", got)
	}
}

func TestReadSecretValueFromStdin(t *testing.T) {
	got, err := readSecretValue(
		"GITHUB_TOKEN",
		"", false, // --value unset
		true,                             // --stdin set
		strings.NewReader("ghp_piped\n"), // pipe with trailing newline
	)
	if err != nil {
		t.Fatalf("readSecretValue: %v", err)
	}
	if got != "ghp_piped" {
		t.Errorf("got %q, want ghp_piped (newline stripped)", got)
	}
}

func TestReadSecretValueFromStdinNoTrailingNewline(t *testing.T) {
	got, err := readSecretValue(
		"GITHUB_TOKEN",
		"", false,
		true,
		strings.NewReader("no_newline"),
	)
	if err != nil {
		t.Fatalf("readSecretValue: %v", err)
	}
	if got != "no_newline" {
		t.Errorf("got %q, want no_newline", got)
	}
}

func TestReadSecretValueMutualExclusion(t *testing.T) {
	_, err := readSecretValue(
		"GITHUB_TOKEN",
		"ghp_literal", true, // --value set
		true, // --stdin also set
		strings.NewReader(""),
	)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error %q does not mention mutual exclusion", err.Error())
	}
}

// The TTY prompt path (last branch) requires a real terminal fd and cannot
// be exercised in a test process without mocking os.Stdin. The non-TTY
// guard at the top of the TTY branch is tested implicitly by the test
// runner, which always sees os.Stdin as non-TTY; calling readSecretValue
// with both flags unset would hit that guard but also mutates global state
// (fmt.Printf to stdout), so we leave it uncovered by direct test.
