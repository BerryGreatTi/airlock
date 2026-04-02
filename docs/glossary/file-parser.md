# File Parser

The `FileParser` interface (`internal/secrets/fileformat.go`) provides a unified abstraction for reading and writing secret files in any supported format. Each format has a dedicated implementation:

| Format | Parser | Extension detection |
|--------|--------|-------------------|
| dotenv | `DotenvParser` | `.env`, `.env.*` |
| JSON | `JSONParser` | `.json` |
| YAML | `YAMLParser` | `.yaml`, `.yml` |
| INI | `INIParser` | `.ini`, `.cfg` |
| Properties | `PropertiesParser` | `.properties` |
| Plain text | `TextParser` | Any unrecognized extension |

The interface has three methods:

- `Format()` -- returns the `FileFormat` constant
- `Parse(path)` -- reads a file and returns a flat list of `SecretEntry` items. Nested structures (JSON, YAML) are flattened using `/` as the path separator (e.g., `db/password`, `servers/0/host`). Only string-typed leaf values produce entries.
- `Write(path, entries)` -- writes entries back to the file atomically (temp file + rename). JSON and YAML parsers read the original file to preserve non-string values, comments, and structure.

Format detection is extension-based via `DetectFormat()`. Users can override with `--format` on CLI commands. The `ResolveParser()` helper combines detection, validation, and parser creation.

See [ADR-0008](../decisions/ADR-0008-multi-format-secrets.md) for the design decisions behind multi-format support.
