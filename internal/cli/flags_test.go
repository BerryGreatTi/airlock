package cli

import "testing"

func TestParseCSVList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty input returns non-nil empty slice", "", []string{}},
		{"single value", "foo", []string{"foo"}},
		{"multiple values", "foo,bar,baz", []string{"foo", "bar", "baz"}},
		{"trims whitespace", " foo , bar ", []string{"foo", "bar"}},
		{"drops empty entries between commas", "foo,,bar", []string{"foo", "bar"}},
		{"drops trailing empty entries", "foo,bar,", []string{"foo", "bar"}},
		{"drops leading empty entries", ",foo,bar", []string{"foo", "bar"}},
		{"all whitespace returns empty", "  ", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCSVList(tc.in)
			if got == nil {
				t.Fatalf("expected non-nil slice for %q, got nil", tc.in)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch for %q: got %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] for %q: got %q, want %q", i, tc.in, got[i], tc.want[i])
				}
			}
		})
	}
}
