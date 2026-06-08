<p align="center">
  <img src="internal/app/assets/icon.png" alt="Conduit" width="128" height="128">
</p>

<h1 align="center">Conduit</h1>

<p align="center">
  Windows desktop utility: local HTTP CONNECT proxy with TLS ClientHello fragmentation and an embedded YouTube viewer (Wails v3 / WebView2).
</p>

<p align="center">
  <a href="https://github.com/foursecondfivefour/conduit/releases/latest"><img src="https://img.shields.io/github/v/release/foursecondfivefour/conduit?label=download&style=flat-square" alt="Latest release"></a>
  <a href="https://github.com/foursecondfivefour/conduit/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/foursecondfivefour/conduit/ci.yml?branch=main&style=flat-square" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License"></a>
</p>

Designed for a small footprint: one WebView window, mobile YouTube UI, full system-tray control. Release builds run without a console window.

## Download

- **Portable:** `conduit.exe` + `conduit-updater.exe` from [Latest release](https://github.com/foursecondfivefour/conduit/releases/latest)
- **Installer:** `Conduit-Setup-1.2.0.exe` (installs to `%LocalAppData%\Programs\Conduit\`)

Requires [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/).

> Unsigned builds may show a SmartScreen prompt on first run. See [docs/CODE_SIGNING.md](docs/CODE_SIGNING.md).

## Features (v1.2.0)

- Persistent settings (DPI, DoH, allowlist, language, autostart, system proxy, minimize-to-tray, auto-update)
- DoH providers: Cloudflare, Google, Quad9
- Allowlist presets: YouTube only, Google media, custom domains (via `preferences.json`)
- DPI strategies: none, TCP segmentation (1/2/8 bytes), TLS record fragmentation (5/2 bytes)
- Connection test, file logging, GitHub auto-update
- Windows: autostart, system proxy with restore on exit, **Ctrl+Shift+Y** to show/hide YouTube
- Russian and English UI

## Requirements

- Windows 10 (amd64)
- [Go 1.25+](https://go.dev/dl/) (to build from source)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)

## Build

```powershell
go generate ./...
go build -ldflags="-s -w -H=windowsgui -X github.com/foursecondfivefour/conduit/internal/config.Version=1.2.0" -tags production -o build\conduit.exe .
go build -ldflags="-s -w -H=windowsgui" -o build\conduit-updater.exe .\cmd\conduit-updater
```

## Run

```powershell
.\build\conduit.exe
```

1. Splash screen while Conduit starts
2. First-launch onboarding (RU/EN)
3. Proxy on `127.0.0.1:31284` (or next free port)
4. YouTube window (`https://m.youtube.com`)
5. Tray: DPI, DNS, domains, options, updates, connection test, language, logs

### Portable mode

```powershell
.\build\conduit.exe -portable
```

Config and logs: `./Conduit/preferences.json`, `./Conduit/logs/conduit.log`

### Proxy only (lowest RAM)

```powershell
.\build\conduit.exe -no-gui
.\build\conduit.exe -no-gui -system-proxy -portable
```

### Logs

- Default: `%AppData%\Conduit\logs\conduit.log`
- Portable: `./Conduit/logs/conduit.log`

Open from tray: **Open log** / **Открыть лог**

## Verify proxy

```powershell
curl.exe -x http://127.0.0.1:31284 https://www.youtube.com -I
```

## Architecture

```
WebView2 → CONNECT proxy → DoH DNS → TCP:443
                ↓
      TLS ClientHello fragmentation (first write only)
```

## Tests

```powershell
go test ./...
```

## Troubleshooting

| Issue | Suggestion |
|-------|------------|
| SmartScreen warning | Expected for unsigned builds; see CODE_SIGNING.md |
| YouTube won't load | Try another DPI strategy; run connection test; flush DNS cache |
| Other apps lose network | Disable system proxy in tray before quitting |
| Auto-update fails | Download manually from GitHub; check antivirus lock on `conduit.exe` |

## Disclaimer

Network filtering and circumvention may be regulated in your jurisdiction. You are responsible for compliance with local law. This project is provided as-is without warranty.

## License

MIT — see [LICENSE](LICENSE).
