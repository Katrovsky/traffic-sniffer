package main

import (
	"strconv"
	"strings"
)

func splitAddress(addr string) (string, string) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, "[") {
		i := strings.LastIndex(addr, "]:")
		if i == -1 {
			return "", ""
		}
		return addr[1:i], addr[i+2:]
	}
	i := strings.LastIndex(addr, ":")
	if i == -1 {
		return "", ""
	}
	return strings.Trim(addr[:i], "[]"), addr[i+1:]
}

func extractPidFromSS(line string) int {
	idx := strings.Index(line, "pid=")
	if idx == -1 {
		return 0
	}
	sub := line[idx+4:]
	end := strings.IndexFunc(sub, func(r rune) bool { return r < '0' || r > '9' })
	if end == -1 {
		end = len(sub)
	}
	pid, _ := strconv.Atoi(sub[:end])
	return pid
}
