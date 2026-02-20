package main

import (
	"slices"
	"strings"
)

func isBackgroundNoise(p procInfo) bool {
	n := strings.ToLower(p.Name)
	noisy := []string{
		"steamwebhelper", "runtimebroker", "widget", "update",
		"backgroundtaskhost", "idle", "system", "svchost",
		"taskhost", "shellexperiencehost", "securityhealthservice",
		"chromecrashhandler",
	}
	for _, b := range noisy {
		if strings.Contains(n, b) {
			return true
		}
	}
	return false
}

func getApps() []string {
	procs := listProcesses()
	seen := map[string]bool{}
	for _, p := range procs {
		if isSystemProcess(p) || isBackgroundNoise(p) {
			continue
		}
		seen[strings.ToLower(p.Name)] = true
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
