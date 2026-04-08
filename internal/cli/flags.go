package cli

import "strings"

// parseCSVList splits a comma-separated CLI flag value into a trimmed,
// non-empty list. Always returns a non-nil slice (empty input → []string{})
// so callers can distinguish "flag set to empty" (empty slice) from "flag
// not present" (nil, handled separately by checking cmd.Flags().Changed).
//
// Empty entries between commas (e.g., "foo,,bar" or trailing ",") are
// silently dropped — the previous hand-rolled implementations across
// commands accidentally produced phantom empty-string entries on those
// inputs.
func parseCSVList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
