package secrets

import "fmt"

// ValidateFormat returns an error if the format string is not a recognized FileFormat.
func ValidateFormat(f FileFormat) error {
	switch f {
	case FormatDotenv, FormatJSON, FormatYAML, FormatINI, FormatProperties, FormatText:
		return nil
	default:
		return fmt.Errorf("unsupported format %q; use dotenv, json, yaml, ini, properties, or text", f)
	}
}
