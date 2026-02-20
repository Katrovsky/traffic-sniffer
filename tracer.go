package main

import (
	"context"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Conn struct {
	IP        string
	Port      string
	Domain    string
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
}

type Tracer struct {
	AppName     string
	PIDs        map[int]bool
	Conns       map[string]*Conn
	DomainCache map[string]string
	Procs       []procInfo
	Mux         sync.RWMutex
}

func newTracer(app string) *Tracer {
	t := &Tracer{
		AppName:     app,
		PIDs:        map[int]bool{},
		Conns:       map[string]*Conn{},
		DomainCache: map[string]string{},
	}
	t.collectPIDs(app)
	return t
}

func (t *Tracer) collectPIDs(app string) {
	procs := listProcesses()
	t.Procs = procs
	for _, p := range procs {
		if strings.ToLower(p.Name) == app {
			t.PIDs[p.PID] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for _, p := range procs {
			if t.PIDs[p.PPID] && !t.PIDs[p.PID] {
				t.PIDs[p.PID] = true
				changed = true
			}
		}
	}
}

func (t *Tracer) scan() {
	if runtime.GOOS == "windows" {
		t.scanWinNetstat()
	} else {
		t.scanLinuxSS()
	}
}

func (t *Tracer) scanWinNetstat() {
	out, err := exec.Command("netstat", "-ano").CombinedOutput()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		fields := strings.Fields(l)
		if len(fields) < 5 {
			continue
		}
		proto := strings.ToUpper(fields[0])
		if proto != "TCP" && proto != "UDP" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
		if err != nil || !t.PIDs[pid] {
			continue
		}
		host, port := splitAddress(fields[2])
		if host == "" || isLoopback(host) {
			continue
		}
		key := host + ":" + port
		if !seen[key] {
			seen[key] = true
			t.add(host, port)
		}
	}
}

func (t *Tracer) scanLinuxSS() {
	out, err := exec.Command("ss", "-tpn").CombinedOutput()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		parts := strings.Fields(l)
		if len(parts) < 5 {
			continue
		}
		pid := extractPidFromSS(l)
		if pid == 0 || !t.PIDs[pid] {
			continue
		}
		host, port := splitAddress(parts[4])
		if host == "" || isLoopback(host) {
			continue
		}
		key := host + ":" + port
		if !seen[key] {
			seen[key] = true
			t.add(host, port)
		}
	}
}

func isLoopback(host string) bool {
	return host == "0.0.0.0" || host == "127.0.0.1" ||
		host == "::" || host == "::1" || host == "[::1]"
}

func (t *Tracer) add(ip, port string) {
	key := ip + ":" + port
	now := time.Now()
	t.Mux.Lock()
	defer t.Mux.Unlock()
	if c, ok := t.Conns[key]; ok {
		c.Count++
		c.LastSeen = now
	} else {
		t.Conns[key] = &Conn{
			IP:        ip,
			Port:      port,
			Domain:    t.resolveAsync(ip),
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
	}
}

func (t *Tracer) resetCounters() {
	t.Mux.Lock()
	for _, c := range t.Conns {
		c.Count = 0
	}
	t.Mux.Unlock()
}

func (t *Tracer) resolveAsync(ip string) string {
	if v, ok := t.DomainCache[ip]; ok {
		return v
	}
	t.DomainCache[ip] = "resolving..."
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		names, _ := net.DefaultResolver.LookupAddr(ctx, ip)
		d := ip
		if len(names) > 0 {
			d = strings.TrimSuffix(names[0], ".")
		}
		t.Mux.Lock()
		t.DomainCache[ip] = d
		for _, c := range t.Conns {
			if c.IP == ip {
				c.Domain = d
			}
		}
		t.Mux.Unlock()
	}()
	return "resolving..."
}
