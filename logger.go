package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	logColIP   = 20
	logColPort = 6
)

type logger struct {
	path         string
	file         *os.File
	loggedKeys   map[string]bool
	maxDomainLen int
}

func newLogger(app string) (*logger, error) {
	ts := time.Now().Format("150405_02.01.06")
	path := fmt.Sprintf("nettracer_%s_%s.log", app, ts)
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	l := &logger{
		path:         path,
		file:         f,
		loggedKeys:   map[string]bool{},
		maxDomainLen: 6,
	}
	l.writeHeader()
	return l, nil
}

func (l *logger) writeHeader() {
	if l.file == nil {
		return
	}
	lineLen := logColIP + 1 + logColPort + 1 + l.maxDomainLen + 1 + 5 + 1 + 8
	header := fmt.Sprintf("%-*s %-*s %-*s %-5s  %s\n",
		logColIP, "IP",
		logColPort, "Port",
		l.maxDomainLen, "Domain",
		"Proto",
		"First seen",
	)
	separator := strings.Repeat("─", lineLen) + "\n"
	l.file.Seek(0, 0)
	l.file.Truncate(int64(len(header) + len(separator)))
	l.file.WriteString(header)
	l.file.WriteString(separator)
}

func (l *logger) flush(t *Tracer) {
	if l.file == nil || t == nil {
		return
	}
	t.Mux.RLock()
	defer t.Mux.RUnlock()

	dirty := false
	for key, c := range t.conns {
		if c.Domain == "resolving..." || l.loggedKeys[key] {
			continue
		}
		if n := utf8.RuneCountInString(c.Domain); n > l.maxDomainLen {
			l.maxDomainLen = n
			dirty = true
		}
	}
	if dirty {
		l.writeHeader()
	}

	for key, c := range t.conns {
		if c.Domain == "resolving..." || l.loggedKeys[key] {
			continue
		}
		fmt.Fprintf(l.file, "%-*s %-*s %-*s %-5s  %s\n",
			logColIP, c.IP,
			logColPort, c.Port,
			l.maxDomainLen, c.Domain,
			c.Proto,
			c.FirstSeen.Format("15:04:05"),
		)
		l.loggedKeys[key] = true
	}
}

func (l *logger) close(t *Tracer) {
	if l.file == nil {
		return
	}
	l.finalFlush(t)
	l.file.Close()
	l.file = nil
}

func (l *logger) finalFlush(t *Tracer) {
	if l.file == nil || t == nil {
		return
	}
	t.Mux.RLock()
	defer t.Mux.RUnlock()

	dirty := false
	for _, c := range t.conns {
		if n := utf8.RuneCountInString(c.Domain); n > l.maxDomainLen {
			l.maxDomainLen = n
			dirty = true
		}
	}
	if dirty {
		l.writeHeader()
	}

	for key, c := range t.conns {
		if l.loggedKeys[key] {
			continue
		}
		if c.Domain == "resolving..." {
			c.Domain = c.IP
		}
		fmt.Fprintf(l.file, "%-*s %-*s %-*s %-5s  %s\n",
			logColIP, c.IP,
			logColPort, c.Port,
			l.maxDomainLen, c.Domain,
			c.Proto,
			c.FirstSeen.Format("15:04:05"),
		)
	}
}
