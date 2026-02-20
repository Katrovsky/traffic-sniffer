# NetTracer

Network connection monitor with a TUI interface. Tracks TCP/UDP connections of a specific application in real time.

## Project structure

```
nettracer/
├── main.go              entry point
├── model.go             state, Init/Update/View, log helpers
├── styles.go            lipgloss styles
├── ui_select.go         application selection screen
├── ui_monitor.go        monitoring screen
├── tracer.go            Tracer: scanning, DNS, connection tracking
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
# Linux — ss -tpn requires elevated privileges
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

## Log file

`nettracer_<app>_<timestamp>.log` is created on start.
Domain column width is dynamic — it grows to fit the longest domain seen in the session, so no data is ever truncated.
