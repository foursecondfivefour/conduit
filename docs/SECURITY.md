# Security

Conduit is a local-first Windows app: a loopback CONNECT proxy, an optional system proxy, and auto-update from GitHub. No telemetry, no remote listener.

## Threat model

| Actor | Capability | Mitigation |
|---|---|---|
| Same-user malware | Use the system proxy or call the loopback proxy | Allowlist-only CONNECT; default YouTube preset; warning when enabling the system proxy |
| Network MITM on update | Replace the downloaded binary | HTTPS + GitHub host allowlist; mandatory SHA256; PE header and size checks |
| Malicious `preferences.json` | Widen the allowlist via `customDomains` | Validation on load; TLD, IP, wildcard, and broad-suffix entries rejected (max 64 entries) |
| Malicious release URL | Open an arbitrary URL from the tray | `ValidateReleaseURL` restricts to `https://github.com/foursecondfivefour/conduit`; `ShellExecute` instead of `cmd /c start` |

Out of scope: remote attackers (the proxy binds `127.0.0.1` only) and Authenticode signing — see [CODE_SIGNING.md](CODE_SIGNING.md).

## Controls

| Area | Control |
|---|---|
| Access control | Allowlist-enforced CONNECT; presets `youtube`, `google_media`, `custom`; default `youtube` |
| Network exposure | Proxy bound to `127.0.0.1`; pprof, when enabled, also localhost only |
| Supply chain | Download hosts limited to `github.com` and `objects.githubusercontent.com`; `govulncheck`, `gosec`, CodeQL in CI |
| Transport | TLS 1.2 minimum for DoH, health checks, and update downloads |
| Integrity | SHA256 sidecar is mandatory and fail-closed; version strings sanitized before path use |
| Data | Settings and logs under `%AppData%\Conduit` (or `.\Conduit` in portable mode); no telemetry |
| Memory bounds | DNS cache capped at 512 hosts; idle timeout on tunnels (120 s); `goleak` in tests |
| Injection / SSRF | JSON preferences sanitized on load; release URLs restricted to this repo; CONNECT targets checked against the allowlist |

## Reporting

Open a [GitHub Security Advisory](https://github.com/foursecondfivefour/conduit/security/advisories/new) or contact the maintainer through the repository.

## User guidance

- Keep the **system proxy** off unless other apps need Conduit.
- Do not add broad custom suffixes (`com`, `google.com`).
- Use official GitHub releases; verify `conduit.exe.sha256` when installing manually.
