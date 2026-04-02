package secrets

// ResolveParser detects or validates the format and returns the appropriate parser.
func ResolveParser(path, formatOverride string) (FileFormat, FileParser, error) {
	var format FileFormat
	if formatOverride != "" {
		format = FileFormat(formatOverride)
		if err := ValidateFormat(format); err != nil {
			return "", nil, err
		}
	} else {
		format = DetectFormat(path)
	}
	return format, ParserFor(format), nil
}
