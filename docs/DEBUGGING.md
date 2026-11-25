# Debugging

## Builds

| Build | Command | Contents |
|---|---|---|
| Debug | `go build -o build/conduit-debug.exe .` | Symbols, Go profiling, DevTools |
| Production | `go build -tags production -o build/conduit.exe .` | Stripped (`-s -w`), no DevTools, no remote debugging port |

Updater: `go build -o build/conduit-updater.exe ./cmd/conduit-updater`.

## Runtime flags

| Flag | Effect |
|---|---|
| `-debug` | slog level `Debug`; WebView DevTools and `--remote-debugging-port=9222` on non-production builds |
| `-debug-inspector` | Open DevTools when the YouTube window is shown (needs `-debug` and a non-production build) |
| `-pprof=:6060` | HTTP pprof on `127.0.0.1:6060` |
| `-memprofile=heap.out` | Write a heap profile on exit |
| `-no-gui` | Proxy only — the right mode for soak and load tests |

```powershell
.\build\conduit-debug.exe -no-gui -pprof=:6060 -debug
.\build\conduit-debug.exe -debug -debug-inspector
```

With `-debug`, debug-level records go to `%AppData%\Conduit\logs\conduit.log`.

## Windows and UI

The UI is four WebView2 windows (Wails v3); there is no frontend bundle.

| Window | Source | Notes |
|---|---|---|
| Splash | `internal/app/ui/splash.html` | Shown at least `SplashDuration` (1 s) |
| Loading placeholder | `internal/app/ui/loading.html` | Dark screen while YouTube preloads hidden |
| Onboarding | `internal/app/ui/onboarding.html` | `AllowSimpleEventEmit: true` for `onboarding:finish` / `onboarding:skip` |
| YouTube | `https://m.youtube.com` | Main window, traffic through the local CONNECT proxy |

### Preload sequence

1. The YouTube window is created hidden with `loading.html`.
2. On the first navigation completion it `SetURL(m.youtube.com)`.
3. On the second completion `youtubeReady` closes.
4. The splash waits for `SplashDuration` plus `youtubeReady` (20 s timeout), then shows YouTube or onboarding.

With `-debug`, each milestone is logged (`placeholder navigation completed`, `youtube navigation completed`, `showing youtube window`).

### DevTools

With `-debug` on a non-production build: DevTools are enabled on splash, onboarding, and YouTube windows; Chromium gets `--remote-debugging-port=9222`; the tray gains **Developer tools** and **Reload YouTube**. Attach via `chrome://inspect` → **Remote Target**.

`-debug-inspector` opens DevTools once YouTube is visible — WebView2 refuses an inspector on a hidden window.

`AllowSimpleEventEmit` is enabled on onboarding only, per Wails guidance.

### Checklist

| Step | Expectation |
|---|---|
| Startup | Logs show placeholder → YouTube navigation, then `showing youtube window` |
| Remote targets | `chrome://inspect` lists the Conduit WebViews |
| Proxy use | YouTube requests go through `127.0.0.1:<port>` |
| Onboarding | Finish/skip emit and log `onboarding finished` / `onboarding skipped` |
| Restore | Minimize to tray and restore without a white flash |

WebView2 memory lives outside the Go heap — use Task Manager for GUI soak runs.

## pprof

```powershell
go tool pprof -http=:8080 http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -http=:8080 http://127.0.0.1:6060/debug/pprof/goroutine
```

## Soak

- **no-gui:** drive many CONNECT requests and watch `goroutine` and `heap`.
- **GUI:** watch YouTube for 10–15 minutes, minimize to tray, quit, compare heap before/after.

## Tests

```powershell
go test ./...
go test -race ./...
```

`go.uber.org/goleak` guards proxy start/stop in `internal/proxy`.

## Symptoms

| Symptom | Likely cause |
|---|---|
| Goroutine count grows | Tray or update loops not stopped; stuck CONNECT relays |
| Heap grows (no-gui) | Unbounded DNS cache (capped at 512 hosts) |
| Hung CONNECT | Missing idle timeout on a tunneled connection |
| White screen on YouTube | Preload not finished before showing; check navigation logs |
| DevTools missing | Built with `-tags production`, or run without `-debug` |
