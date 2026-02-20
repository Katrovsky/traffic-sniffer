//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

type procInfo struct {
	PID  int
	PPID int
	Name string
	Path string
}

var (
	modKernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot   = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = modKernel32.NewProc("Process32FirstW")
	procProcess32NextW             = modKernel32.NewProc("Process32NextW")
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
)

type processEntry32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [260]uint16
}

func listProcesses() []procInfo {
	h, _, _ := procCreateToolhelp32Snapshot.Call(0x2, 0)
	if h == 0 {
		return nil
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	r, _, _ := procProcess32FirstW.Call(h, uintptr(unsafe.Pointer(&e)))
	if r == 0 {
		return nil
	}
	var out []procInfo
	for {
		name := syscall.UTF16ToString(e.ExeFile[:])
		path := getWinProcessPath(int(e.ProcessID))
		out = append(out, procInfo{
			PID:  int(e.ProcessID),
			PPID: int(e.ParentProcessID),
			Name: strings.TrimSuffix(strings.ToLower(name), ".exe"),
			Path: strings.ToLower(path),
		})
		r, _, _ = procProcess32NextW.Call(h, uintptr(unsafe.Pointer(&e)))
		if r == 0 {
			break
		}
	}
	return out
}

func getWinProcessPath(pid int) string {
	h, err := syscall.OpenProcess(0x1000|0x0400, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(h), 0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}

func isSystemProcess(p procInfo) bool {
	if p.PID <= 4 {
		return true
	}
	return strings.Contains(strings.ToLower(p.Path), `windows\system32`)
}
