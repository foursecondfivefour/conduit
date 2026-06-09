# Debugging Conduit

## Debug build

Release builds use `-tags production` and strip symbols (`-s -w`). For Go profiling, stack traces, and **frontend WebView debugging**, build **without** the production tag:

```powershell
go build -o build/conduit-debug.exe .
go build -o build/conduit-updater.exe ./cmd/conduit-updater
```

Production build (no DevTools, no remote debugging port):

```powershell
go build -tags production -o build/conduit.exe .
```

## Runtime flags

| Flag | Description |
|------|-------------|
| `-debug` | slog level `Debug`; enables WebView DevTools and `--remote-debugging-port=9222` |
| `-debug-inspector` | Auto-open inspector on the YouTube window at startup (requires `-debug`, non-production build) |
| `-pprof=:6060` | HTTP pprof on `127.0.0.1:6060` (localhost only) |
| `-memprofile=heap.out` | Write heap profile on exit |
| `-no-gui` | Proxy only (best for soak / load tests) |

Example (backend only):

```powershell
.\build\conduit-debug.exe -no-gui -pprof=:6060 -debug
```

Example (GUI with frontend debug):

```powershell
.\build\conduit-debug.exe -debug
# optional: auto-open inspector on YouTube
.\build\conduit-debug.exe -debug -debug-inspector
```

Logs with `-debug` go to `%LocalAppData%\Conduit\logs\`.

## Frontend / WebView2

Conduit has no separate React/Vue bundle. The UI is four WebView2 windows (Wails v3):

| Window | Source | Notes |
|--------|--------|-------|
| Splash | `internal/app/ui/splash.html` | Shown ~1s at startup |
| Loading placeholder | `internal/app/ui/loading.html` | Dark screen while YouTube preloads hidden |
| Onboarding | `internal/app/ui/onboarding.html` | `AllowSimpleEventEmit: true` for `onboarding:finish` / `onboarding:skip` |
| YouTube | `https://m.youtube.com` | Main window; traffic via local CONNECT proxy |

### Preload pipeline

1. YouTube window starts hidden with `loading.html`.
2. On first `WebViewNavigationCompleted`, `SetURL(m.youtube.com)`.
3. On second navigation complete, `youtubeReady` closes.
4. Splash waits for `SplashDuration` + `youtubeReady` (20s timeout), then shows YouTube or onboarding.

With `-debug`, navigation milestones are logged at `slog.Debug` (placeholder ready, YouTube ready, splash close).

### DevTools and remote debugging

When `-debug` is set (non-production build):

- `DevToolsEnabled: true` on splash, onboarding, and YouTube windows.
- `--remote-debugging-port=9222` is added to Chromium args (localhost only).
- Tray menu adds **Developer tools** (`OpenDevTools`) and **Reload YouTube**.

Attach from Chrome/Edge: open `chrome://inspect` → **Remote Target** → pick a Conduit WebView.

`-debug-inspector` opens DevTools when the YouTube window is shown (not during hidden preload — WebView2 rejects inspector on hidden windows). Requires a non-production build.

### Onboarding events

Onboarding HTML emits:

- `onboarding:finish` — complete tutorial
- `onboarding:skip` — skip (first run only marks complete when appropriate)

Handled in `internal/app/startup.go` via `app.Event.On`. `AllowSimpleEventEmit` is enabled **only** on onboarding (not on YouTube/loading) per Wails security guidance.

### Frontend debug checklist

**A. Startup (splash + loading + YouTube)**

1. Run `conduit-debug.exe -debug` with a clean `%LocalAppData%\Conduit` if testing first-run.
2. Confirm logs: `placeholder navigation completed` → `youtube navigation completed` before `showing youtube window`.
3. `chrome://inspect` should list WebView targets.
4. Network tab on YouTube: requests to `m.youtube.com` via `127.0.0.1:<proxy-port>`.

**B. Onboarding**

1. Clear `onboardingComplete` in preferences or use first launch.
2. DevTools → Console: `window.wails.Events` available.
3. Finish/skip buttons → debug logs `onboarding finished` / `onboarding skipped`.

**C. YouTube**

1. No long white screen after show (dark placeholder preloads first).
2. Video playback through proxy; check Console for errors.
3. Minimize to tray → restore without flash.

WebView2 memory is outside the Go heap — use Task Manager for GUI soak.

References: [Wails v3 window options](https://v3.wails.io).

## pprof

With `-pprof=:6060` running:

```powershell
go tool pprof -http=:8080 http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -http=:8080 http://127.0.0.1:6060/debug/pprof/goroutine
```

## Soak scenarios

**no-gui:** run many CONNECT requests through the proxy; watch `goroutine` and `heap` for growth.

**GUI:** use YouTube 10–15 minutes, minimize to tray, quit; compare heap before/after. WebView2 native memory is not visible in Go heap — use Task Manager for GUI soak.

## Tests

```powershell
go test ./...
go test -race ./...
```

`go.uber.org/goleak` guards proxy start/stop in `internal/proxy`.

## Common symptoms

| Symptom | Likely cause |
|---------|----------------|
| Goroutine count grows | Tray/update loops before stop; stuck CONNECT relays |
| Heap grows (no-gui) | Unbounded DNS cache (fixed: cap 512 hosts) |
| Hung CONNECT | Missing idle timeout on tunneled connections |
| White screen on YouTube | Preload not finished before show; check debug nav logs |
| DevTools missing | Built with `-tags production` or run without `-debug` |
