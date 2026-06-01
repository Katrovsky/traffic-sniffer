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

const connTTL = 45 * time.Second

type Conn struct {
	IP        string
	Port      string
	Proto     string
	Domain    string
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
}

type Tracer struct {
	AppName string
	Mux     sync.RWMutex

	pids        map[int]bool
	conns       map[string]*Conn
	domainCache map[string]string
}

func newTracer(app string) *Tracer {
	t := &Tracer{
		AppName:     app,
		pids:        map[int]bool{},
		conns:       map[string]*Conn{},
		domainCache: map[string]string{},
	}
	t.refreshPIDs()
	return t
}

func (t *Tracer) refreshPIDs() {
	procs := listProcesses()
	next := map[int]bool{}
	for _, p := range procs {
		if strings.ToLower(p.Name) == t.AppName {
			next[p.PID] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for _, p := range procs {
			if next[p.PPID] && !next[p.PID] {
				next[p.PID] = true
				changed = true
			}
		}
	}
	t.Mux.Lock()
	t.pids = next
	t.Mux.Unlock()
}

func (t *Tracer) PIDCount() int {
	t.Mux.RLock()
	defer t.Mux.RUnlock()
	return len(t.pids)
}

func (t *Tracer) scan() {
	t.refreshPIDs()
	if runtime.GOOS == "windows" {
		t.scanWinNetstat()
	} else {
		t.scanLinuxSS()
	}
	t.evict()
}

func (t *Tracer) evict() {
	cutoff := time.Now().Add(-connTTL)
	t.Mux.Lock()
	defer t.Mux.Unlock()
	for k, c := range t.conns {
		if c.LastSeen.Before(cutoff) {
			delete(t.conns, k)
		}
	}
}

func (t *Tracer) scanWinNetstat() {
	out, err := exec.Command("netstat", "-ano").CombinedOutput()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for l := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(l)
		if len(fields) < 5 {
			continue
		}
		proto := strings.ToUpper(fields[0])
		if proto != "TCP" && proto != "UDP" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
		if err != nil {
			continue
		}
		t.Mux.RLock()
		hasPID := t.pids[pid]
		t.Mux.RUnlock()
		if !hasPID {
			continue
		}
		host, port := splitAddress(fields[2])
		if host == "" || isLoopback(host) {
			continue
		}
		key := proto + ":" + host + ":" + port
		if !seen[key] {
			seen[key] = true
			t.add(proto, host, port)
		}
	}
}

func (t *Tracer) scanLinuxSS() {
	seen := map[string]bool{}
	for _, args := range [][]string{
		{"-tpn"},
		{"-upn"},
	} {
		out, err := exec.Command("ss", args...).CombinedOutput()
		if err != nil {
			continue
		}
		proto := "TCP"
		if args[0] == "-upn" {
			proto = "UDP"
		}
		for l := range strings.SplitSeq(string(out), "\n") {
			parts := strings.Fields(l)
			if len(parts) < 5 {
				continue
			}
			pid := extractPidFromSS(l)
			if pid == 0 {
				continue
			}
			t.Mux.RLock()
			hasPID := t.pids[pid]
			t.Mux.RUnlock()
			if !hasPID {
				continue
			}
			host, port := splitAddress(parts[4])
			if host == "" || isLoopback(host) {
				continue
			}
			key := proto + ":" + host + ":" + port
			if !seen[key] {
				seen[key] = true
				t.add(proto, host, port)
			}
		}
	}
}

func isLoopback(host string) bool {
	return host == "0.0.0.0" || host == "127.0.0.1" ||
		host == "::" || host == "::1" || host == "[::1]"
}

func (t *Tracer) add(proto, ip, port string) {
	key := proto + ":" + ip + ":" + port
	now := time.Now()

	t.Mux.Lock()
	defer t.Mux.Unlock()

	if c, ok := t.conns[key]; ok {
		c.Count++
		c.LastSeen = now
		return
	}

	domain := t.cachedDomain(ip)
	t.conns[key] = &Conn{
		IP:        ip,
		Port:      port,
		Proto:     proto,
		Domain:    domain,
		Count:     1,
		FirstSeen: now,
		LastSeen:  now,
	}
}

func (t *Tracer) cachedDomain(ip string) string {
	if v, ok := t.domainCache[ip]; ok {
		return v
	}
	t.domainCache[ip] = "resolving..."
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		names, _ := net.DefaultResolver.LookupAddr(ctx, ip)
		d := ip
		if len(names) > 0 {
			d = strings.TrimSuffix(names[0], ".")
		}
		t.Mux.Lock()
		t.domainCache[ip] = d
		for _, c := range t.conns {
			if c.IP == ip {
				c.Domain = d
			}
		}
		t.Mux.Unlock()
	}()
	return "resolving..."
}

func (t *Tracer) resetCounters() {
	t.Mux.Lock()
	for _, c := range t.conns {
		c.Count = 0
	}
	t.Mux.Unlock()
}

func (t *Tracer) snapshot() map[string]*Conn {
	t.Mux.RLock()
	defer t.Mux.RUnlock()
	cp := make(map[string]*Conn, len(t.conns))
	for k, v := range t.conns {
		c := *v
		cp[k] = &c
	}
	return cp
}
