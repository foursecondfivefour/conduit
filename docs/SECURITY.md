# Security

Conduit is a **local-first** Windows app: a loopback CONNECT proxy, optional system proxy, and auto-update from GitHub.

## Threat model

| Actor | Capability | Mitigation |
|-------|------------|------------|
| Same-user malware | Use system proxy / call loopback proxy | Allowlist-only CONNECT; default YouTube preset; proxy warning when enabling system proxy |
| Network MITM on update | Replace downloaded binary | HTTPS + GitHub host allowlist; **mandatory** SHA256; PE header/size checks |
| Malicious `preferences.json` | Broad allowlist via `customDomains` | Validation on load; reject TLD/broad suffixes |
| Malicious release page URL | Open arbitrary URL from tray | `ValidateReleaseURL`; `ShellExecute` instead of `cmd /c start` |

Out of scope: remote attackers (proxy binds `127.0.0.1` only), Authenticode signing (planned; see `docs/CODE_SIGNING.md`).

## OWASP Top 10:2025 mapping (v1.2.1)

| ID | Category | Control |
|----|----------|---------|
| A01 | Broken access control | Custom domain validation; default `youtube` preset |
| A02 | Security misconfiguration | System proxy warning; prefs sanitization |
| A03 | Supply chain | Download URL allowlist (`github.com`, `objects.githubusercontent.com`) |
| A05 | Injection | Safe URL open; JSON prefs sanitization |
| A08 | Integrity failures | Required SHA256; updater path validation |
| A10 | SSRF | Release URLs restricted to repo paths |

## Reporting

Open a [GitHub Security Advisory](https://github.com/foursecondfivefour/conduit/security/advisories/new) or email the maintainer via the repository contact.

## User guidance

- Keep **system proxy** off unless you need other apps to use Conduit.
- Do not add broad custom suffixes (e.g. `com`, `google.com`).
- Prefer official GitHub releases; verify `conduit.exe.sha256` when installing manually.
