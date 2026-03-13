//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type procInfo struct {
	PID  int
	PPID int
	Name string
	Path string
}

func listProcesses() []procInfo {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []procInfo
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		name := procName(pid)
		if name == "" {
			continue
		}
		ppid := procPPID(pid)
		path, _ := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		out = append(out, procInfo{
			PID:  pid,
			PPID: ppid,
			Name: name,
			Path: strings.ToLower(path),
		})
	}
	return out
}

func procName(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func procPPID(pid int) int {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "PPid:"); ok {
			ppid, _ := strconv.Atoi(strings.TrimSpace(after))
			return ppid
		}
	}
	return 0
}

func isSystemProcess(p procInfo) bool {
	if p.PID <= 2 {
		return true
	}
	systemPaths := []string{"/usr/lib/systemd", "/lib/systemd", "/sbin/", "/usr/sbin/"}
	for _, sp := range systemPaths {
		if strings.HasPrefix(p.Path, sp) {
			return true
		}
	}
	return false
}
