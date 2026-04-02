# Secret Entry

A `SecretEntry` (`internal/secrets/fileformat.go`) represents a single key-value pair extracted from any secret file format. It is the unified data type that flows through the encryption pipeline, replacing the older format-specific `EnvEntry` (which remains for backward compatibility with `EnvScanner`).

```go
type SecretEntry struct {
    Path      string // slash-separated key path: "db/password"
    Value     string // the value (plaintext or ENC[age:...])
    Encrypted bool   // true if Value matches the ENC pattern
}
```

Key properties:

- **Path uses `/` separator**: Nested keys from JSON/YAML are flattened with `/` (not `.`). For example, `{"db": {"password": "x"}}` becomes `Path: "db/password"`. Array elements use numeric indices: `servers/0/host`. Flat formats (dotenv, INI, properties) use the key name directly as the path.
- **Encrypted flag**: Set by `SetEncryptedFlags()` during parsing and by `EncryptSelected()` after encryption. Derived from `crypto.IsEncrypted(Value)`.
- **Format-agnostic**: The same type is used across all 6 formats. The proxy mapping (`ENC[age:...] -> plaintext`) is also format-agnostic.

`LeafKey(path)` extracts the last segment of a path for heuristic matching: `LeafKey("db/password")` returns `"password"`.

See [File Parser](file-parser.md) for how entries are produced and [ADR-0008](../decisions/ADR-0008-multi-format-secrets.md) for design decisions.
