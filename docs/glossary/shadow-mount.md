# Shadow Mount

A file-level Docker bind mount that overlays a specific file with an encrypted replacement. When the container reads the file at the shadowed path, it sees the encrypted version (`ENC[age:...]` ciphertext) instead of the original plaintext.

Shadow mounts are the mechanism that connects the Scanner pipeline (which discovers and encrypts secrets) to the container runtime. Each shadow mount replaces one file:

```
Host tmpdir:                    Container sees:
/tmp/airlock-xxx/settings.json  ->  /home/airlock/.claude/settings.json  (encrypted)
/tmp/airlock-xxx/proj-settings  ->  /workspace/.claude/settings.json     (encrypted)
/tmp/airlock-xxx/env.enc        ->  /workspace/.env                      (encrypted)
```

Key properties:

- **File-level**: Shadow mounts target individual files, not directories. Only files that contain detected secrets are shadowed.
- **Bind mount precedence**: Docker bind mounts take precedence over volume mounts at the same path. When `~/.claude` is a writable named volume, shadow mounts still correctly overlay specific files within it. The container process cannot access the underlying plaintext at the shadowed path.
- **Read-only**: Shadow mounts are always mounted with `:ro` to prevent the container from modifying the encrypted files.
- **Ephemeral**: Shadow files live in a temporary directory on the host. They are created fresh on each `run`/`start` and deleted when the session ends.
- **Scope limitation**: Only files that were scanned and found to contain secrets are shadowed. Other files in the same directory (e.g., `history.jsonl`, session data) are not affected by shadow mounts and remain accessible in their original form.

See [ADR-0005](../decisions/ADR-0005-settings-secret-protection.md) for the Scanner pipeline design and [ADR-0006](../decisions/ADR-0006-writable-claude-volume.md) for how shadow mounts interact with the writable volume.
