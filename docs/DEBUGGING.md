# Debugging Conduit

## Debug build

Release builds strip symbols (`-s -w`). For profiling and stack traces, build without strip:

```powershell
go build -tags production -o build/conduit-debug.exe .
go build -o build/conduit-updater.exe ./cmd/conduit-updater
```

## Runtime flags

| Flag | Description |
|------|-------------|
| `-debug` | slog level `Debug` |
| `-pprof=:6060` | HTTP pprof on `127.0.0.1:6060` (localhost only) |
| `-memprofile=heap.out` | Write heap profile on exit |
| `-no-gui` | Proxy only (best for soak / load tests) |

Example:

```powershell
.\build\conduit-debug.exe -no-gui -pprof=:6060 -debug
```

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
