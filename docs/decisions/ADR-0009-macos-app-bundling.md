# ADR-0009: macOS app bundling with embedded Go CLI

## Status

Accepted

## Context

Airlock's macOS GUI (SwiftUI) invokes the Go CLI as a subprocess for all container orchestration, encryption, and Docker operations (see ADR-0004). Until now, these are separate binaries: users install the GUI app and must separately install the CLI binary on their PATH. The release CI workflow creates a basic `.app` bundle but has two problems:

1. **CLIService never checks the bundle.** `resolveAirlockBinary()` searches `$PATH` and falls back to `/usr/local/bin/airlock`. The Go binary placed at `Contents/Resources/bin/airlock` by CI is invisible to the running app -- a dead binary that ships with every release.

2. **No local packaging.** Developers cannot build a `.app` bundle locally. The packaging logic is inline in the GitHub Actions workflow, not reusable.

The goal is a self-contained `Airlock.app` that bundles both binaries, requires no separate CLI installation, and can be built locally via `make gui-package`.

### Distribution model

GitHub Releases as a ZIP (and optionally DMG). Not the Mac App Store.

### Sandboxing evaluation

macOS App Sandbox was evaluated and rejected for Airlock. Sandbox restricts apps to their own container directory and requires explicit entitlements for system access. Airlock's core operations are fundamentally incompatible:

- **Docker socket access**: requires reading `/var/run/docker.sock` or runtime-specific sockets (`~/.rd/docker.sock`, `~/.colima/docker.sock`)
- **Arbitrary project directories**: user workspaces can be anywhere on disk
- **`~/.claude` config access**: for import/export operations
- **Secret file access**: registered files anywhere on the filesystem
- **Subprocess execution**: spawning Go CLI and Docker processes

Apps distributed outside the Mac App Store (via GitHub Releases, Homebrew, DMG) are not required to be sandboxed. This is the standard approach for developer tools like Docker Desktop, iTerm2, and VS Code.

## Decision

### 1. Embed Go CLI as sibling executable in Contents/MacOS/

Place the Go binary at `Airlock.app/Contents/MacOS/airlock`, next to the Swift binary at `Contents/MacOS/AirlockApp`. This follows Apple's convention that `Contents/MacOS/` contains executables and `Contents/Resources/` contains data files.

```
Airlock.app/Contents/
  Info.plist
  MacOS/
    AirlockApp    (Swift GUI -- CFBundleExecutable)
    airlock       (Go CLI -- helper binary)
  Resources/
    AppIcon.icns
```

### 2. Fix CLIService binary resolution

Update `resolveAirlockBinary()` to check the app bundle first:

1. Explicit `binaryPath` (settings override)
2. `Bundle.main.executableURL` sibling (`Contents/MacOS/airlock`)
3. `Bundle.main.resourceURL` fallback (`Contents/Resources/bin/airlock` -- legacy CI path)
4. `$PATH` search (works for `swift run` during development)
5. Fallback: `/usr/local/bin/airlock`

Each candidate is verified with `FileManager.isExecutableFile(atPath:)` before returning. When running via `swift run` (no real bundle), steps 2-3 fail gracefully and fall through to PATH search.

### 3. Inside-out code signing (ad-hoc by default)

Sign the embedded Go CLI helper first, then the outer app bundle:

```sh
codesign --force --sign - Airlock.app/Contents/MacOS/airlock
codesign --force --sign - Airlock.app
```

This is the Apple-recommended "inside-out" signing order -- each outer signature seals the already-signed nested code. The deprecated `codesign --deep` flag is avoided because `notarytool` explicitly rejects `--deep`-signed bundles.

Ad-hoc signing (`--sign -`) requires no Apple Developer account and is sufficient for local use. Users downloading from GitHub will need to bypass Gatekeeper on first launch (right-click > Open, or System Settings > Privacy & Security > Open Anyway).

Future enhancement: Developer ID signing + notarization ($99/year Apple Developer Program) removes the Gatekeeper warning entirely. The packaging script accepts `--sign <identity>` and automatically enables `--options=runtime --timestamp` (hardened runtime + secure timestamp, both required for notarization) when a non-ad-hoc identity is passed. No code changes needed to move from ad-hoc to notarization-ready signing.

### 4. Local packaging via make gui-package

A `scripts/package-app.sh` script handles the full pipeline: build Go CLI, build Swift GUI, generate `.icns` from the programmatic icon, create `.app` structure, inject version into Info.plist, and code sign. The Makefile exposes this as `make gui-package`.

### 5. Icon generation from existing Canvas rendering

The dock icon is currently rendered programmatically via SwiftUI Canvas at runtime (`AppIconView`). For the `.app` bundle, an `.icns` file is generated at build time by a standalone Swift script that duplicates the Canvas drawing logic and renders to all required sizes (16px through 1024px). The duplication is necessary because the `AirlockApp` SPM target is an `@main` executable and cannot be imported as a library.

## Consequences

### Easier

- **Zero-config installation**: Download `.app`, drag to `/Applications`, done. No separate CLI install needed.
- **Local builds**: Developers can `make gui-package` to produce a testable `.app` bundle.
- **Version consistency**: GUI and CLI versions always match (built together).
- **CI simplification**: Release workflow uses the same script as local builds.

### Harder

- **Icon maintenance**: The Canvas drawing logic exists in two files (`AppIconView.swift` for runtime dock icon, `scripts/generate-icon-main.swift` for `.icns` generation). Changes must be synchronized.
- **Binary size**: The `.app` bundle includes both the Swift GUI (~7MB) and Go CLI (~15MB). This is comparable to other developer tools.
- **Gatekeeper warnings**: Without Developer ID signing + notarization, first-launch requires manual bypass. This is the standard experience for unsigned developer tools.

## Alternatives Considered

### Keep CLI separate (status quo)

Rejected. Requires users to install two things. The bundled CLI in the current release is broken (never found by CLIService). The "GUI includes the CLI engine internally" claim in the installation guide is false without this fix.

### Place Go binary in Contents/Resources/bin/

Rejected. `Contents/Resources/` is for data files per Apple convention. Executables belong in `Contents/MacOS/`. The Resources path also requires an additional path component in resolution logic.

### Use Xcode project instead of SPM + packaging script

Deferred. Opening `Package.swift` in Xcode can generate `.app` bundles with full signing UI, but adds Xcode project files to the repository, complicates CI, and requires all contributors to use Xcode. The script approach is simpler and CI-friendly. Can be revisited if App Store distribution is ever considered.

### Sandbox with temporary entitlements

Rejected. The number of entitlements needed (file access, network, subprocess execution) would effectively negate the sandbox. Non-App-Store apps are not required to be sandboxed. No developer tools in this category (Docker Desktop, VS Code, iTerm2) use sandboxing.

### Universal binary for Swift GUI

Deferred. The Go CLI already builds as a universal binary (arm64 + amd64 via `lipo`). The Swift GUI currently builds for the host architecture only. Universal Swift builds can be added later if Intel Mac support is needed (macOS 14+ requirement already limits to relatively recent hardware).
