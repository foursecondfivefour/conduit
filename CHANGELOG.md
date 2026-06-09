# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.2.1] - 2026-06-09

### Security

- Reject broad custom allowlist suffixes (e.g. `com`, `google.com`); default preset is now **YouTube only**.
- Auto-update: GitHub download URL allowlist, mandatory SHA256 verification, sanitized version paths.
- Hardened `conduit-updater` path checks; release links validated before opening in browser.
- System proxy toggle shows a warning about local open-proxy scope.

### Added

- Debug flags: `-pprof`, `-memprofile`, `-debug`; `docs/DEBUGGING.md`.
- `go.uber.org/goleak` tests; `go test -race` in CI.
- `docs/SECURITY.md` threat model and OWASP mapping.

### Fixed

- Tray/update scheduler and status loop stop on quit; hotkey loop exits on shutdown.
- Proxy `IdleTimeout` and per-read idle deadlines on CONNECT tunnels.
- DNS cache bounded (512 hosts) with eviction.
- File log handle closed on exit.

## [1.2.0] - 2026-06-08

### Added

- Persistent `preferences.json` (strategy, DoH, allowlist, language, window size, autostart, system proxy, minimize-to-tray, auto-update settings) with portable mode (`-portable`).
- Russian/English UI (system tray, splash, onboarding) with language submenu.
- DoH provider selection: Cloudflare, Google, Quad9.
- Allowlist presets: YouTube only, Google media, custom domains via JSON.
- Six DPI strategies including none, TCP 1/2/8-byte segmentation, and TLS record fragmentation variants.
- Connection health check through the local proxy.
- Rotating file logs under `%AppData%\Conduit\logs` or `./Conduit/logs` in portable mode.
- Windows autostart, system proxy with snapshot restore, minimize-to-tray on close, Ctrl+Shift+Y hotkey toggle.
- GitHub auto-update: check, download, SHA256 verify, `conduit-updater.exe` replace + relaunch.
- Inno Setup installer (`Conduit-Setup-1.2.0.exe`).
- Integration tests for CONNECT allowlist behavior.
- Issue/PR templates and `docs/CODE_SIGNING.md`.

### Changed

- System tray expanded with status line, DNS/domains/options/updates submenus.
- Release CI builds `conduit.exe`, `conduit-updater.exe`, SHA256 checksum, and installer.

## [1.1.0] - 2026-06-07

### Added

- Splash screen and first-run onboarding.

## [1.0.0] - 2026-06-07

### Added

- Initial release: local CONNECT proxy, TLS fragmentation, Wails YouTube viewer, system tray.
