# Security

Conduit is a **local-first** Windows app: a loopback CONNECT proxy, optional system proxy, and auto-update from GitHub.

## Threat model

| Actor | Capability | Mitigation |
|-------|------------|------------|
| Same-user malware | Use system proxy / call loopback proxy | Allowlist-only CONNECT; default YouTube preset; proxy warning when enabling system proxy |
| Network MITM on update | Replace downloaded binary | HTTPS + GitHub host allowlist; **mandatory** SHA256; PE header/size checks |
| Malicious `preferences.json` | Broad allowlist via `customDomains` | Validation on load; reject TLD/broad suffixes |
| Malicious release page URL | Open arbitrary URL from tray | `ValidateReleaseURL`; `ShellExecute` instead of `cmd /c start` |

Out of scope: remote attackers (proxy binds `127.0.0.1` only), Authenticode signing (see `docs/CODE_SIGNING.md`).

## OWASP Top 10:2025 mapping (v1.3.0)

| ID | Category | Control |
|----|----------|---------|
| A01 | Broken access control | Custom domain validation; default `youtube` preset; split tunnel enforcement |
| A02 | Security misconfiguration | System proxy warning; prefs sanitization; SOCKS5/MASQUE localhost-only defaults |
| A03 | Supply chain | Download URL allowlist (`github.com`, `objects.githubusercontent.com`); dependency scanning (govulncheck) |
| A04 | Insecure design | AI strategy fallback to safe default; rate-limited health checks |
| A05 | Injection | Safe URL open; JSON prefs sanitization; `ShellExecute` hardened |
| A06 | Memory safety | Go memory-safe runtime; `sync.Pool` bounds; DNS cache cap (512) |
| A07 | Cryptographic failures | TLS 1.2 minimum; SHA256 verification; ECH support |
| A08 | Integrity failures | Required SHA256; updater path validation; reproducible builds (`-buildid=`) |
| A09 | Data exposure | Localhost-only proxy; no telemetry; user-data in `%AppData%` only |
| A10 | SSRF | Release URLs restricted to repo paths; upstream proxy validation |

## Reporting

Open a [GitHub Security Advisory](https://github.com/foursecondfivefour/conduit/security/advisories/new) or email the maintainer via the repository contact.

## User guidance

- Keep **system proxy** off unless you need other apps to use Conduit.
- Do not add broad custom suffixes (e.g. `com`, `google.com`).
- Prefer official GitHub releases; verify `conduit.exe.sha256` when installing manually.
