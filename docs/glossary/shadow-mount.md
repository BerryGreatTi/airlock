# Shadow Mount

A file-level Docker bind mount that overlays a specific file with an encrypted replacement. When the container reads the file at the shadowed path, it sees the encrypted version (`ENC[age:...]` ciphertext) instead of the original plaintext.

Shadow mounts are the mechanism that connects the Scanner pipeline (which discovers and encrypts secrets) to the container runtime. Each shadow mount replaces one file:

```
Host tmpdir:                    Container sees:
/tmp/airlock-xxx/settings.json  ->  /home/airlock/.claude/settings.json       (encrypted)
/tmp/airlock-xxx/proj-settings  ->  /workspace/my-app/.claude/settings.json   (encrypted)
/tmp/airlock-xxx/env.enc        ->  /workspace/my-app/.env                    (encrypted)
```

Container workspace paths use the project directory basename (e.g., `/workspace/my-app`), so shadow mounts target the correct per-project paths.

Key properties:

- **File-level**: Shadow mounts target individual files, not directories. Only files that contain detected secrets are shadowed.
- **Bind mount precedence**: Docker bind mounts take precedence over volume mounts at the same path. When `~/.claude` is a writable named volume, shadow mounts still correctly overlay specific files within it. The container process cannot access the underlying plaintext at the shadowed path.
- **Read-only**: Shadow mounts are always mounted with `:ro` to prevent the container from modifying the encrypted files.
- **Ephemeral**: Shadow files live in a temporary directory on the host. They are created fresh on each `run`/`start` and deleted when the session ends.
- **Scope limitation**: Only files that were scanned and found to contain secrets are shadowed. Other files in the same directory (e.g., `history.jsonl`, session data) are not affected by shadow mounts and remain accessible in their original form.

Shadow mounts are produced by three scanner types:

- **ClaudeScanner** -- shadows `~/.claude/settings.json` and project-level settings
- **EnvScanner** -- shadows `.env` files (via `--env` flag)
- **FileScanner** -- shadows any user-registered secret file (JSON, YAML, INI, properties, text) configured in `.airlock/config.yaml`

For files outside the workspace, `FileScanner` mounts to `/run/airlock/files/<index>-<filename>` instead of a workspace-relative path.

See [ADR-0005](../decisions/ADR-0005-settings-secret-protection.md) for the Scanner pipeline design, [ADR-0008](../decisions/ADR-0008-multi-format-secrets.md) for multi-format support, and [ADR-0006](../decisions/ADR-0006-writable-claude-volume.md) for how shadow mounts interact with the writable volume.
