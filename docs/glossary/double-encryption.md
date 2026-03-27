# Double Encryption

A bug pattern where an already-encrypted `ENC[age:...]` value is encrypted a second time, producing a nested ciphertext like `ENC[age:ENCRYPT(ENC[age:ORIGINAL])]`.

In the airlock proxy pipeline, this causes the proxy to peel only one encryption layer, delivering `ENC[age:ORIGINAL]` (still encrypted) to the destination instead of the plaintext secret.

## How it occurred

The GUI "Encrypt All" action encrypts `.env` values in-place. When "Activate" subsequently calls `airlock start --env .env`, the CLI's `EncryptEntries` function re-encrypted the already-encrypted values.

## Resolution

`EncryptEntries` now checks each value with `crypto.IsEncrypted()`. If the value already matches the `ENC[age:...]` pattern, it decrypts with the private key to obtain plaintext for the proxy mapping, and preserves the original ciphertext unchanged.
