# NetTracer

Network connection monitor with a TUI interface. Tracks TCP and UDP connections of a specific application in real time.

## Project structure

```
nettracer/
├── main.go              entry point
├── model.go             state, Init/Update/View, status helpers
├── styles.go            lipgloss styles
├── ui_select.go         application selection screen
├── ui_monitor.go        monitoring screen
├── tracer.go            Tracer: scanning, DNS, PID tracking, TTL eviction
├── logger.go            log file: header, flush, final flush on close
├── process_linux.go     process listing — Linux  (build tag !windows)
├── process_windows.go   process listing — Windows (build tag windows)
├── process_common.go    filtering, getApps()
├── util.go              splitAddress, extractPidFromSS
└── go.mod
```

## Build

```bash
go mod tidy
go build -o nettracer .
```

## Run

```bash
# Linux — ss requires elevated privileges
sudo ./nettracer

# Windows — run cmd/PowerShell as Administrator
nettracer.exe
```

## Key bindings

| Key | Action |
| ----- | -------- |
| `↑` / `↓` | navigate |
| `Enter` | select application |
| `s` | cycle sort order: hits → ip → port → domain |
| `r` | reset hit counters |
| `b` / `Esc` | back to application list |
| `q` / `Ctrl+C` | quit |

## Behaviour notes

- **PID tracking**: re-scanned on every tick — new child processes are picked up automatically.
- **Dead connections**: evicted after 45 s of silence (configurable via `connTTL` in `tracer.go`).
- **UDP**: tracked on both Linux (`ss -upn`) and Windows (`netstat -ano`).
- **Proto column**: each row shows TCP or UDP.
- **Warnings**: if no PIDs are found for the selected app, a `⚠ no PIDs found` banner appears in the header.
- **Empty state**: table shows a hint while waiting for first traffic.
- **Port sort**: numeric, not lexicographic.
- **Log file**: `nettracer_<app>_<timestamp>.log` — written on first resolution, flushed with final counts on quit/back.
  Domain column width is dynamic and never truncates data.
