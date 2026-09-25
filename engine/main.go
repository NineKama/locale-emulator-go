//go:build windows && (amd64 || 386) && cgo

// This DLL owns a Go runtime. InstallLocale is explicitly called after
// LoadLibrary returns; no hooks or Go callbacks are installed from DllMain.
package main

import "C"

import (
	"encoding/binary"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel = windows.NewLazySystemDLL("kernel32.dll")
var once sync.Once
var result C.uint

//export InstallLocale
func InstallLocale(_ unsafe.Pointer) C.uint {
	once.Do(func() { result = C.uint(install()) })
	return result
}

func install() uint32 {
	// Match resolved IAT pointers, including API-set imports, without parsing
	// untrusted strings. Only the main executable's normal imports are changed.
	hooks := map[string]uint32{
		"GetACP": 932, "GetOEMCP": 932,
		"GetUserDefaultLCID": 0x411, "GetSystemDefaultLCID": 0x411, "GetThreadLocale": 0x411,
		"MultiByteToWideChar": 0, "WideCharToMultiByte": 0,
	}
	memory, _, _ := kernel.NewProc("VirtualAlloc").Call(0, 4096, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if memory == 0 {
		return 0
	}
	// Kept for the process lifetime because IAT entries point into this page.
	codePage := unsafe.Slice((*byte)(unsafe.Pointer(memory)), 4096)
	replacements := map[uintptr]uintptr{}
	offset := 0
	for name, value := range hooks {
		p := kernel.NewProc(name)
		if p.Find() != nil {
			return 0
		}
		code := stub(p.Addr(), value)
		copy(codePage[offset:], code)
		replacements[p.Addr()] = memory + uintptr(offset)
		offset += 32
	}
	getError := kernel.NewProc("GetLastError")
	setError := kernel.NewProc("SetLastError")
	if getError.Find() != nil || setError.Find() != nil {
		return 0
	}
	for _, h := range ansiHooks() {
		p := h.dll.NewProc(h.name)
		if p.Find() != nil {
			return 0
		}
		code := ansiBridge(ansiCallback(h), getError.Addr(), setError.Addr(), h.argc)
		if offset+len(code) > len(codePage) {
			return 0
		}
		copy(codePage[offset:], code)
		replacements[p.Addr()] = memory + uintptr(offset)
		offset = (offset + len(code) + 15) &^ 15
	}
	// Publish executable code only after all bytes are written (RW, then RX).
	var old uint32
	if windows.VirtualProtect(memory, 4096, windows.PAGE_EXECUTE_READ, &old) != nil {
		return 0
	}
	current, _, _ := kernel.NewProc("GetCurrentProcess").Call()
	kernel.NewProc("FlushInstructionCache").Call(current, memory, 4096)
	base, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	if base == 0 {
		return 0
	}
	header := unsafe.Slice((*byte)(unsafe.Pointer(base)), 4096)
	nt := int(binary.LittleEndian.Uint32(header[0x3c:]))
	if nt < 0 || nt+152 > len(header) || string(header[nt:nt+4]) != "PE\x00\x00" {
		return 0
	}
	opt := nt + 24
	if binary.LittleEndian.Uint16(header[opt:]) != peMagic {
		return 0
	}
	size := int(binary.LittleEndian.Uint32(header[opt+56:]))
	if size < 4096 {
		return 0
	}
	image := unsafe.Slice((*byte)(unsafe.Pointer(base)), size)
	dir := int(binary.LittleEndian.Uint32(header[opt+importOffset:]))
	dirSize := int(binary.LittleEndian.Uint32(header[opt+importOffset+4:]))
	if dir == 0 || dirSize < 20 || dir > size-dirSize {
		return 0
	}
	// Import descriptors have the same layout on both architectures; IAT slots
	// contain architecture-sized pointers. Only the main EXE is patched.
	count := uint32(0)
	for d := dir; d+20 <= dir+dirSize; d += 20 {
		thunk := int(binary.LittleEndian.Uint32(image[d+16:]))
		if thunk == 0 {
			break
		}
		for slot := thunk; slot > 0 && slot <= size-int(unsafe.Sizeof(uintptr(0))); slot += int(unsafe.Sizeof(uintptr(0))) {
			old := *(*uintptr)(unsafe.Pointer(base + uintptr(slot)))
			if old == 0 {
				break
			}
			replacement, ok := replacements[old]
			if !ok {
				continue
			}
			address := base + uintptr(slot)
			var protect uint32
			if windows.VirtualProtect(address, unsafe.Sizeof(uintptr(0)), windows.PAGE_READWRITE, &protect) != nil {
				return 0
			}
			*(*uintptr)(unsafe.Pointer(address)) = replacement
			var ignored uint32
			if windows.VirtualProtect(address, unsafe.Sizeof(uintptr(0)), protect, &ignored) != nil {
				return 0
			}
			count++
		}
	}
	return count
}
func main() {}
