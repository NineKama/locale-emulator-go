//go:build windows && (amd64 || 386) && cgo

package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Bridge frames live on the native stack: [0] is the return value, [1] is
// LastError, and [2:] contains Win32 arguments. Each bridge restores LastError
// AFTER returning from Go, since runtime transitions may change thread state.
type ansiHook struct {
	dll  *windows.LazyDLL
	name string
	argc int
	run  func([]uintptr)
}

var mbWide = kernel.NewProc("MultiByteToWideChar")
var wideMB = kernel.NewProc("WideCharToMultiByte")
var commandOnce sync.Once
var commandANSI uintptr
var commandError uintptr

func errCode(e error) uintptr {
	if n, ok := e.(syscall.Errno); ok {
		return uintptr(n)
	}
	return 87
}

// wide decodes a null-terminated CP932 string, including its terminator.
// Bound path buffers to avoid unbounded allocations inside a callback.
func wide(s uintptr) ([]uint16, uintptr) {
	if s == 0 {
		return nil, 87
	}
	n, _, e := mbWide.Call(932, 0, s, ^uintptr(0), 0, 0)
	if n == 0 {
		return nil, errCode(e)
	}
	if n > 32768 {
		return nil, 206
	}
	out := make([]uint16, n)
	r, _, e := mbWide.Call(932, 0, s, ^uintptr(0), uintptr(unsafe.Pointer(&out[0])), n)
	if r == 0 {
		return nil, errCode(e)
	}
	return out, 0
}

// ansi converts exactly the supplied UTF-16 units. Reject lossy substitution
// instead of silently turning an executable filename into question marks.
func ansi(w []uint16) ([]byte, uintptr) {
	if len(w) == 0 {
		return []byte{0}, 0
	}
	var used uint32
	n, _, e := wideMB.Call(932, 0, uintptr(unsafe.Pointer(&w[0])), uintptr(len(w)), 0, 0, 0, uintptr(unsafe.Pointer(&used)))
	if n == 0 {
		return nil, errCode(e)
	}
	out := make([]byte, n)
	r, _, e := wideMB.Call(932, 0, uintptr(unsafe.Pointer(&w[0])), uintptr(len(w)), uintptr(unsafe.Pointer(&out[0])), n, 0, uintptr(unsafe.Pointer(&used)))
	if r == 0 {
		return nil, errCode(e)
	}
	if used != 0 {
		return nil, 1113
	}
	return out, 0
}
func widePath(proc *windows.LazyProc, prefix ...uintptr) ([]uint16, uintptr) {
	out := make([]uint16, 32768)
	args := append(prefix, uintptr(unsafe.Pointer(&out[0])), uintptr(len(out)))
	n, _, e := proc.Call(args...)
	runtime.KeepAlive(out)
	if n == 0 {
		return nil, errCode(e)
	}
	if n >= uintptr(len(out)) {
		return nil, 206
	}
	return out[:n+1], 0
}
func fail(f []uintptr, ret, err uintptr) { f[0] = ret; f[1] = err }

// ANSI paths use Unicode Windows APIs explicitly. Hooking GetACP in the EXE
// alone cannot change conversions performed internally by Windows.
func ansiHooks() []ansiHook {
	user := windows.NewLazySystemDLL("user32.dll")
	return []ansiHook{
		{kernel, "CreateFileA", 7, func(f []uintptr) {
			w, e := wide(f[2])
			if e != 0 {
				fail(f, ^uintptr(0), e)
				return
			}
			r, _, err := kernel.NewProc("CreateFileW").Call(uintptr(unsafe.Pointer(&w[0])), f[3], f[4], f[5], f[6], f[7], f[8])
			runtime.KeepAlive(w)
			f[0] = r
			f[1] = errCode(err)
		}},
		{kernel, "GetFileAttributesA", 1, func(f []uintptr) {
			w, e := wide(f[2])
			if e != 0 {
				fail(f, 0xffffffff, e)
				return
			}
			r, _, err := kernel.NewProc("GetFileAttributesW").Call(uintptr(unsafe.Pointer(&w[0])))
			runtime.KeepAlive(w)
			f[0] = r
			if uint32(r) == 0xffffffff {
				f[1] = errCode(err)
			}
		}},
		{kernel, "GetModuleFileNameA", 3, func(f []uintptr) {
			if f[4] == 0 || f[3] == 0 {
				fail(f, 0, 122)
				return
			}
			w, e := widePath(kernel.NewProc("GetModuleFileNameW"), f[2])
			if e != 0 {
				fail(f, 0, e)
				return
			}
			b, e := ansi(w)
			if e != 0 {
				fail(f, 0, e)
				return
			}
			n := uintptr(len(b))
			if n > f[4] {
				n = f[4]
				fail(f, n, 122)
			} else {
				f[0] = n - 1
			}
			out := unsafe.Slice((*byte)(unsafe.Pointer(f[3])), int(n))
			copy(out, b)
			out[len(out)-1] = 0
		}},
		{kernel, "GetCommandLineA", 0, func(f []uintptr) {
			commandOnce.Do(func() {
				p, _, _ := kernel.NewProc("GetCommandLineW").Call()
				if p == 0 {
					commandError = 87
					return
				}
				n := 0
				for ; n < 32768; n++ {
					if *(*uint16)(unsafe.Pointer(p + uintptr(n*2))) == 0 {
						break
					}
				}
				if n == 32768 {
					commandError = 206
					return
				}
				b, e := ansi(unsafe.Slice((*uint16)(unsafe.Pointer(p)), n+1))
				if e != 0 {
					commandError = e
					return
				}
				commandANSI, _, _ = kernel.NewProc("VirtualAlloc").Call(0, uintptr(len(b)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
				if commandANSI == 0 {
					commandError = 8
					return
				}
				copy(unsafe.Slice((*byte)(unsafe.Pointer(commandANSI)), len(b)), b)
			})
			f[0] = commandANSI
			if commandError != 0 {
				f[1] = commandError
			}
		}},
		{kernel, "GetCurrentDirectoryA", 2, func(f []uintptr) {
			out := make([]uint16, 32768)
			n, _, err := kernel.NewProc("GetCurrentDirectoryW").Call(uintptr(len(out)), uintptr(unsafe.Pointer(&out[0])))
			if n == 0 {
				fail(f, 0, errCode(err))
				return
			}
			if n >= uintptr(len(out)) {
				fail(f, 0, 206)
				return
			}
			b, e := ansi(out[:n+1])
			if e != 0 {
				fail(f, 0, e)
				return
			}
			if uintptr(len(b)) > f[2] {
				f[0] = uintptr(len(b))
				return
			}
			if f[3] == 0 {
				fail(f, 0, 87)
				return
			}
			copy(unsafe.Slice((*byte)(unsafe.Pointer(f[3])), len(b)), b)
			f[0] = uintptr(len(b) - 1)
		}},
		{kernel, "GetFullPathNameA", 4, func(f []uintptr) {
			w, e := wide(f[2])
			if e != 0 {
				fail(f, 0, e)
				return
			}
			out := make([]uint16, 32768)
			var part uintptr
			n, _, err := kernel.NewProc("GetFullPathNameW").Call(uintptr(unsafe.Pointer(&w[0])), uintptr(len(out)), uintptr(unsafe.Pointer(&out[0])), uintptr(unsafe.Pointer(&part)))
			runtime.KeepAlive(w)
			if n == 0 {
				fail(f, 0, errCode(err))
				return
			}
			if n >= uintptr(len(out)) {
				fail(f, 0, 206)
				return
			}
			b, e := ansi(out[:n+1])
			if e != 0 {
				fail(f, 0, e)
				return
			}
			if f[5] != 0 {
				*(*uintptr)(unsafe.Pointer(f[5])) = 0
			}
			if uintptr(len(b)) > f[3] {
				f[0] = uintptr(len(b))
				return
			}
			if f[4] == 0 {
				fail(f, 0, 87)
				return
			}
			copy(unsafe.Slice((*byte)(unsafe.Pointer(f[4])), len(b)), b)
			f[0] = uintptr(len(b) - 1)
			if f[5] != 0 && part != 0 {
				chars := (part - uintptr(unsafe.Pointer(&out[0]))) / 2
				if chars == 0 {
					*(*uintptr)(unsafe.Pointer(f[5])) = f[4]
				} else {
					prefix, e := ansi(out[:chars])
					if e == 0 {
						*(*uintptr)(unsafe.Pointer(f[5])) = f[4] + uintptr(len(prefix))
					}
				}
			}
		}},
		{user, "MessageBoxA", 4, func(f []uintptr) {
			var text, title []uint16
			var ptext, ptitle uintptr
			if f[3] != 0 {
				text, _ = wide(f[3])
				if len(text) > 0 {
					ptext = uintptr(unsafe.Pointer(&text[0]))
				}
			}
			if f[4] != 0 {
				title, _ = wide(f[4])
				if len(title) > 0 {
					ptitle = uintptr(unsafe.Pointer(&title[0]))
				}
			}
			r, _, err := user.NewProc("MessageBoxW").Call(f[2], ptext, ptitle, f[5])
			runtime.KeepAlive(text)
			runtime.KeepAlive(title)
			f[0] = r
			if r == 0 {
				f[1] = errCode(err)
			}
		}},
	}
}

func ansiCallback(h ansiHook) uintptr {
	return syscall.NewCallback(func(frame uintptr) uintptr {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		f := unsafe.Slice((*uintptr)(unsafe.Pointer(frame)), h.argc+2)
		h.run(f)
		return 0
	})
}
