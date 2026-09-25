//go:build windows && (amd64 || 386)

package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel = windows.NewLazySystemDLL("kernel32.dll")
var launchMu sync.Mutex
var engineLib *windows.DLL

func modules(pid uint32) ([]windows.ModuleEntry32, error) {
	h, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE, pid)
	if e != nil {
		return nil, e
	}
	defer windows.CloseHandle(h)
	m := windows.ModuleEntry32{Size: uint32(unsafe.Sizeof(windows.ModuleEntry32{}))}
	if e = windows.Module32First(h, &m); e != nil {
		return nil, e
	}
	var out []windows.ModuleEntry32
	for {
		out = append(out, m)
		e = windows.Module32Next(h, &m)
		if e == windows.ERROR_NO_MORE_FILES {
			break
		}
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}

// Resolve by module + RVA rather than assuming identical ASLR addresses.
func remoteAddress(pid uint32, address uintptr) (uintptr, error) {
	local, e := modules(uint32(os.Getpid()))
	if e != nil {
		return 0, e
	}
	remote, e := modules(pid)
	if e != nil {
		return 0, e
	}
	for _, l := range local {
		if address >= l.ModBaseAddr && address-l.ModBaseAddr < uintptr(l.ModBaseSize) {
			for _, r := range remote {
				if strings.EqualFold(windows.UTF16ToString(l.ExePath[:]), windows.UTF16ToString(r.ExePath[:])) {
					return r.ModBaseAddr + (address - l.ModBaseAddr), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("Could not find the matching module in the target process")
}

func callRemote(process windows.Handle, address, arg uintptr) (uint32, error) {
	h, _, e := kernel.NewProc("CreateRemoteThread").Call(uintptr(process), 0, 0, address, arg, 0, 0)
	if h == 0 {
		return 0, fmt.Errorf("CreateRemoteThread: %w", e)
	}
	defer windows.CloseHandle(windows.Handle(h))
	state, e := windows.WaitForSingleObject(windows.Handle(h), 15000)
	if e != nil {
		return 0, e
	}
	if state != windows.WAIT_OBJECT_0 {
		return 0, fmt.Errorf("The engine did not respond within 15 seconds")
	}
	var code uint32
	ok, _, e := kernel.NewProc("GetExitCodeThread").Call(h, uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return 0, e
	}
	return code, nil
}

// Launch handles a newly created process of the same architecture. Start
// routes cross-architecture requests to a matching helper before calling this.
func Launch(target, dll string) (uint32, error) {
	return launch(target, dll, false)
}

func launch(target, dll string, native bool) (uint32, error) {
	launchMu.Lock()
	defer launchMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	arch, e := Architecture(target)
	if e != nil {
		return 0, e
	}
	if arch != runtime.GOARCH {
		return 0, fmt.Errorf("launcher %s cannot run target %s", runtime.GOARCH, arch)
	}
	var install *windows.Proc
	if !native {
		dll, e = filepath.Abs(dll)
		if e != nil {
			return 0, e
		}
		if _, e = os.Stat(dll); e != nil {
			return 0, fmt.Errorf("Engine DLL missing beside the application: %w", e)
		}
		// Never unload a Go DLL while its runtime threads are alive.
		if engineLib == nil {
			engineLib, e = windows.LoadDLL(dll)
			if e != nil {
				return 0, fmt.Errorf("Load engine: %w", e)
			}
		}
		install, e = engineLib.FindProc("InstallLocale")
		if e != nil {
			return 0, e
		}
	}
	app, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return 0, e
	}
	cmd, e := windows.UTF16PtrFromString(syscall.EscapeArg(target))
	if e != nil {
		return 0, e
	}
	cwd, e := windows.UTF16PtrFromString(filepath.Dir(target))
	if e != nil {
		return 0, e
	}
	si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var pi windows.ProcessInformation
	if e = windows.CreateProcess(app, cmd, nil, nil, false, windows.DEBUG_ONLY_THIS_PROCESS, nil, cwd, &si, &pi); e != nil {
		return 0, e
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	success := false
	debugging := true
	defer func() {
		if !success {
			// A partial hook installation must not leave a suspended target behind.
			windows.TerminateProcess(pi.Process, 1)
			if debugging {
				kernel.NewProc("DebugActiveProcessStop").Call(uintptr(pi.ProcessId))
			}
			windows.WaitForSingleObject(pi.Process, 5000)
		}
	}()
	var imageBase uintptr
	if e = waitForLoader(pi, &imageBase); e != nil {
		return 0, e
	}
	debugging = false
	if native {
		if e = installNative386(pi.Process, pi.ProcessId, imageBase, target); e != nil {
			return 0, e
		}
		if _, e = windows.ResumeThread(pi.Thread); e != nil {
			return 0, e
		}
		success = true
		return pi.ProcessId, nil
	}
	path, e := windows.UTF16FromString(dll)
	if e != nil {
		return 0, e
	}
	n := uintptr(len(path) * 2)
	mem, _, e := kernel.NewProc("VirtualAllocEx").Call(uintptr(pi.Process), 0, n, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if mem == 0 {
		return 0, e
	}
	// Failed launches terminate the process, releasing even timed-out allocations.
	if e = windows.WriteProcessMemory(pi.Process, mem, (*byte)(unsafe.Pointer(&path[0])), n, nil); e != nil {
		return 0, e
	}
	load := kernel.NewProc("LoadLibraryW")
	if e = load.Find(); e != nil {
		return 0, e
	}
	remoteLoad, e := remoteAddress(pi.ProcessId, load.Addr())
	if e != nil {
		return 0, e
	}
	if _, e = callRemote(pi.Process, remoteLoad, mem); e != nil {
		return 0, fmt.Errorf("Load target DLL: %w", e)
	}
	remoteInstall, e := remoteAddress(pi.ProcessId, install.Addr())
	if e != nil {
		return 0, fmt.Errorf("DLL was not loaded: %w", e)
	}
	threadEntry, cleanup, e := installThreadEntry(pi.Process, remoteInstall)
	if e != nil {
		return 0, e
	}
	count, e := callRemote(pi.Process, threadEntry, 0)
	// A timed-out thread may still execute the adapter. Termination frees it.
	if e == nil {
		cleanup()
	}
	if e != nil {
		return 0, e
	}
	if count == 0 || count > 10000 {
		return 0, fmt.Errorf("The engine could not install hooks (code %d); the application may not import any supported APIs", count)
	}
	kernel.NewProc("VirtualFreeEx").Call(uintptr(pi.Process), mem, 0, windows.MEM_RELEASE)
	if _, e = windows.ResumeThread(pi.Thread); e != nil {
		return 0, e
	}
	success = true
	return pi.ProcessId, nil
}

// Break at the EXE entry point, after loader initialization has returned.
// Suspend the primary thread there, detach, then install outside loader lock.
// TLS callbacks / dependency DllMain may already have run: deliberately outside
// the compatibility promise of this MVP.
func waitForLoader(pi windows.ProcessInformation, imageBase *uintptr) error {
	type debugEvent struct {
		Code, PID, TID uint32
		Union          [164]byte // includes 4-byte alignment padding on AMD64
	}
	deadline := time.Now().Add(15 * time.Second)
	var entry uintptr
	var original byte
	firstBreakpoint := true
	patch := func(value byte) error {
		var old uint32
		if e := windows.VirtualProtectEx(pi.Process, entry, 1, windows.PAGE_EXECUTE_READWRITE, &old); e != nil {
			return e
		}
		e := windows.WriteProcessMemory(pi.Process, entry, &value, 1, nil)
		var ignored uint32
		restore := windows.VirtualProtectEx(pi.Process, entry, 1, old, &ignored)
		kernel.NewProc("FlushInstructionCache").Call(uintptr(pi.Process), entry, 1)
		if e != nil {
			return e
		}
		return restore
	}
	for time.Now().Before(deadline) {
		var ev debugEvent
		ok, _, err := kernel.NewProc("WaitForDebugEvent").Call(uintptr(unsafe.Pointer(&ev)), 1000)
		if ok == 0 {
			if err == windows.ERROR_SEM_TIMEOUT {
				continue
			}
			return fmt.Errorf("WaitForDebugEvent: %w", err)
		}
		data := ev.Union[debugPadding:]
		status := uintptr(0x00010002) // DBG_CONTINUE
		initial := false
		switch ev.Code {
		case 3, 6: // CREATE_PROCESS / LOAD_DLL: hFile is first union field.
			h := *(*windows.Handle)(unsafe.Pointer(&data[0]))
			if h != 0 {
				windows.CloseHandle(h)
			}
			if ev.Code == 3 {
				// CREATE_PROCESS_DEBUG_INFO gives the actual mapped image base,
				// independent of short paths, aliases or non-ASCII file names.
				*imageBase = *(*uintptr)(unsafe.Pointer(&data[3*unsafe.Sizeof(uintptr(0))]))
				entry = *(*uintptr)(unsafe.Pointer(&data[debugEntryOffset]))
				if entry == 0 {
					return fmt.Errorf("No entry point found")
				}
				if e := windows.ReadProcessMemory(pi.Process, entry, &original, 1, nil); e != nil {
					return e
				}
				if e := patch(0xcc); e != nil {
					return e
				}
			}
		case 1:
			code := *(*uint32)(unsafe.Pointer(&data[0]))
			address := *(*uintptr)(unsafe.Pointer(&data[exceptionAddressOffset]))
			initial = code == 0x80000003 && ev.TID == pi.ThreadId && address == entry
			if code == 0x80000003 && firstBreakpoint && !initial {
				firstBreakpoint = false
			} else if !initial {
				status = 0x80010001
			} // DBG_EXCEPTION_NOT_HANDLED
		case 5:
			kernel.NewProc("ContinueDebugEvent").Call(uintptr(ev.PID), uintptr(ev.TID), status)
			return fmt.Errorf("The application exited before the engine was loaded")
		}
		if initial {
			if e := patch(original); e != nil {
				return e
			}
			if e := rewindThread(pi.Thread, entry); e != nil {
				return e
			}
			r, _, e := kernel.NewProc("SuspendThread").Call(uintptr(pi.Thread))
			if uint32(r) == 0xffffffff {
				return e
			}
		}
		ok, _, err = kernel.NewProc("ContinueDebugEvent").Call(uintptr(ev.PID), uintptr(ev.TID), status)
		if ok == 0 {
			return err
		}
		if initial {
			ok, _, err = kernel.NewProc("DebugActiveProcessStop").Call(uintptr(pi.ProcessId))
			if ok == 0 {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("Timed out waiting for the Windows loader")
}

// CONTEXT layouts differ, but each helper only handles its own architecture.
func rewindThread(thread windows.Handle, entry uintptr) error {
	storage := make([]byte, contextSize+16)
	var pin runtime.Pinner
	pin.Pin(&storage[0])
	defer pin.Unpin()
	ctx := (uintptr(unsafe.Pointer(&storage[0])) + 15) &^ uintptr(15)
	*(*uint32)(unsafe.Pointer(ctx + contextFlagsOffset)) = contextControl
	ok, _, e := kernel.NewProc("GetThreadContext").Call(uintptr(thread), ctx)
	if ok == 0 {
		return e
	}
	*(*uintptr)(unsafe.Pointer(ctx + contextIPOffset)) = entry
	ok, _, e = kernel.NewProc("SetThreadContext").Call(uintptr(thread), ctx)
	if ok == 0 {
		return e
	}
	return nil
}
