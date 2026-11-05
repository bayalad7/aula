package main

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID   = user32.NewProc("GetWindowThreadProcessId")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procIsIconic             = user32.NewProc("IsIconic")
	procShowWindow           = user32.NewProc("ShowWindow")
	procPostMessageW         = user32.NewProc("PostMessageW")
	procGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	procGetWindow            = user32.NewProc("GetWindow")
)

// processMemoryCountersEx is PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCountersEx struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type proc struct {
	pid, parent uint32
	name        string
}

func processes() (map[uint32]proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)
	out := map[uint32]proc{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[e.ProcessID] = proc{pid: e.ProcessID, parent: e.ParentProcessID, name: windows.UTF16ToString(e.ExeFile[:])}
	}
	return out, nil
}

// tree returns root and every process that descends from it (WebView2 runs
// as msedgewebview2.exe children of the agent).
func tree(root uint32) ([]proc, error) {
	all, err := processes()
	if err != nil {
		return nil, err
	}
	in := map[uint32]bool{root: true}
	for changed := true; changed; {
		changed = false
		for _, p := range all {
			if !in[p.pid] && in[p.parent] && p.pid != p.parent {
				in[p.pid], changed = true, true
			}
		}
	}
	var out []proc
	for pid := range in {
		if p, ok := all[pid]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// usage returns the CPU time (100 ns units) and private memory (bytes) of p.
func usage(p proc) (cpu uint64, private uint64, ok bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.pid)
	if err != nil {
		return 0, 0, false
	}
	defer windows.CloseHandle(h)
	var c, x, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &x, &k, &u); err != nil {
		return 0, 0, false
	}
	cpu = uint64(k.HighDateTime)<<32 | uint64(k.LowDateTime) + uint64(u.HighDateTime)<<32 | uint64(u.LowDateTime)
	var m processMemoryCountersEx
	m.cb = uint32(unsafe.Sizeof(m))
	if r, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.cb)); r != 0 {
		private = uint64(m.PrivateUsage)
	}
	return cpu, private, true
}

type result struct {
	cpu            float64 // % of one core
	ramAvg, ramMax float64 // MB
	procs          int
}

// measure samples the process tree of root every second for d. tick, if set,
// runs after each sample with the elapsed time.
func measure(root int, d time.Duration, tick func(time.Duration) bool) (result, error) {
	var (
		r        result
		samples  int
		startCPU = map[uint32]uint64{}
		lastCPU  = map[uint32]uint64{}
	)
	start := time.Now()
	for {
		ps, err := tree(uint32(root))
		if err != nil {
			return r, err
		}
		var ram uint64
		for _, p := range ps {
			cpu, priv, ok := usage(p)
			if !ok {
				continue
			}
			if _, seen := startCPU[p.pid]; !seen {
				startCPU[p.pid] = cpu
			}
			lastCPU[p.pid] = cpu
			ram += priv
		}
		mb := float64(ram) / (1 << 20)
		r.ramAvg += mb
		samples++
		if mb > r.ramMax {
			r.ramMax = mb
		}
		if len(ps) > r.procs {
			r.procs = len(ps)
		}
		elapsed := time.Since(start)
		if elapsed >= d {
			var used uint64
			for pid, c := range lastCPU {
				used += c - startCPU[pid]
			}
			r.cpu = float64(used) / float64(elapsed/100) * 100 // 100 ns units over wall time
			r.ramAvg /= float64(samples)
			return r, nil
		}
		if tick != nil {
			tick(elapsed)
		}
		time.Sleep(time.Second)
	}
}

type fgInfo struct {
	pid  uint32
	name string
}

func (f fgInfo) String() string { return fmt.Sprintf("%s (pid %d)", f.name, f.pid) }

// inTree reports whether the foreground process is root or one of its
// descendants.
func (f fgInfo) inTree(root int) bool {
	ps, err := tree(uint32(root))
	if err != nil {
		return false
	}
	for _, p := range ps {
		if p.pid == f.pid {
			return true
		}
	}
	return false
}

// foreground returns the process that owns the foreground window. It only
// reads: it never changes which window is in front.
func foreground() fgInfo {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return fgInfo{name: "ninguna"}
	}
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	all, err := processes()
	if err != nil {
		return fgInfo{pid: pid, name: "?"}
	}
	return fgInfo{pid: pid, name: all[pid].name}
}

// aboveForeground reports whether a visible window of root's process tree is
// above the foreground window in the z-order (it covers what the student is
// using, even without taking the keyboard focus). It only reads.
func aboveForeground(root int) bool {
	ps, err := tree(uint32(root))
	if err != nil {
		return false
	}
	mine := map[uint32]bool{}
	for _, p := range ps {
		mine[p.pid] = true
	}
	fg, _, _ := procGetForegroundWindow.Call()
	found := false
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if hwnd == fg {
			return 0 // reached the foreground window first: nothing of ours above it
		}
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		if ic, _, _ := procIsIconic.Call(hwnd); ic != 0 {
			return 1
		}
		var pid uint32
		procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if mine[pid] {
			found = true
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0) // top-level windows, top of the z-order first
	return found
}

// windowsAlive lists the top-level windows of the agent process that have a
// taskbar button (not tool windows, not owned): its Wails window.
func windowsAlive(pid int) []uintptr {
	var out []uintptr
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var p uint32
		procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&p)))
		if int(p) != pid {
			return 1
		}
		if ex, _, _ := procGetWindowLongPtrW.Call(hwnd, ^uintptr(19)); ex&0x80 != 0 { // WS_EX_TOOLWINDOW
			return 1
		}
		if o, _, _ := procGetWindow.Call(hwnd, 4); o != 0 { // GW_OWNER
			return 1
		}
		out = append(out, hwnd)
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out
}

// agentWindow is the visible main window of the agent.
func agentWindow(pid int) (uintptr, error) {
	for _, w := range windowsAlive(pid) {
		if visible(w) {
			return w, nil
		}
	}
	return 0, fmt.Errorf("el agente %d no tiene una ventana visible", pid)
}

func visible(w uintptr) bool { r, _, _ := procIsWindowVisible.Call(w); return r != 0 }
func iconic(w uintptr) bool  { r, _, _ := procIsIconic.Call(w); return r != 0 }

// minimize and closeWindow act on the agent's own window, like its buttons;
// they do not send keyboard or mouse input.
func minimize(w uintptr)    { procShowWindow.Call(w, 6) }            // SW_MINIMIZE
func closeWindow(w uintptr) { procPostMessageW.Call(w, 0x10, 0, 0) } // WM_CLOSE

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}
