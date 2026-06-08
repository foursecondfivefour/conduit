# Conduit

Windows desktop utility: a local HTTP CONNECT proxy with TLS ClientHello fragmentation and an embedded YouTube viewer (Wails v3 / WebView2).

Designed for a small footprint: one WebView window, mobile YouTube UI, system-tray settings.

## Requirements

- Windows 10 (amd64)
- [Go 1.22+](https://go.dev/dl/)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)

## Build

```powershell
go build -ldflags="-s -w" -tags production -o build\conduit.exe .
```

## Run

```powershell
.\build\conduit.exe
```

1. Proxy listens on `127.0.0.1:31284` (or the next free port).
2. A YouTube window opens (`https://m.youtube.com`).
3. Tray icon: DPI strategy, DNS cache reset, quit.

### Proxy only (lowest RAM)

```powershell
.\build\conduit.exe -no-gui
```

Point your browser at `http://127.0.0.1:31284`.

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

The proxy binds to `127.0.0.1` and allows only YouTube/Google media domains.

## Tests

```powershell
go test ./...
```

## Disclaimer

Network filtering and circumvention may be regulated in your jurisdiction. You are responsible for compliance with local law. This project is provided as-is without warranty.

## License

MIT — see [LICENSE](LICENSE).
