package secrets

import (
	"testing"
)

func TestResolveParserAutoDetect(t *testing.T) {
	format, parser, err := ResolveParser("/path/to/config.json", "")
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatJSON {
		t.Errorf("format = %q, want json", format)
	}
	if parser.Format() != FormatJSON {
		t.Errorf("parser format = %q, want json", parser.Format())
	}
}

func TestResolveParserOverride(t *testing.T) {
	format, parser, err := ResolveParser("/path/to/secrets", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatYAML {
		t.Errorf("format = %q, want yaml", format)
	}
	if parser.Format() != FormatYAML {
		t.Errorf("parser format = %q, want yaml", parser.Format())
	}
}

func TestResolveParserInvalidFormat(t *testing.T) {
	_, _, err := ResolveParser("/path/to/file", "bogus")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestResolveParserAllFormats(t *testing.T) {
	formats := []string{"dotenv", "json", "yaml", "ini", "properties", "text"}
	for _, f := range formats {
		t.Run(f, func(t *testing.T) {
			format, parser, err := ResolveParser("/any/path", f)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", f, err)
			}
			if string(format) != f {
				t.Errorf("format = %q, want %q", format, f)
			}
			if parser == nil {
				t.Fatal("parser is nil")
			}
		})
	}
}
