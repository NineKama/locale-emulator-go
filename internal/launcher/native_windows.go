package launcher

import (
	"encoding/binary"
	"fmt"
	"sort"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"locale-emulator-go/internal/native386"
)

// installNative386 writes a small native payload and patches only the main EXE's
// in-memory IAT. No file is changed and no Go DLL/runtime is loaded in the target.
// Failure is handled by Launch, which terminates its newly created process.
func installNative386(process windows.Handle, pid uint32, base uintptr, target string) error {
	if base == 0 {
		return fmt.Errorf("native x86: target module not found")
	}
	var imageSize uint32 = 4096
	read := func(offset uint32, size uint32) ([]byte, error) {
		if size == 0 || offset > imageSize || size > imageSize-offset {
			return nil, fmt.Errorf("native x86: invalid PE range")
		}
		b := make([]byte, size)
		e := windows.ReadProcessMemory(process, base+uintptr(offset), &b[0], uintptr(size), nil)
		return b, e
	}
	header, err := read(0, 4096)
	if err != nil {
		return err
	}
	nt := binary.LittleEndian.Uint32(header[0x3c:])
	if nt > 4096-160 || string(header[nt:nt+4]) != "PE\x00\x00" || binary.LittleEndian.Uint16(header[nt+24:]) != 0x10b {
		return fmt.Errorf("native x86: invalid PE32 header")
	}
	imageSize = binary.LittleEndian.Uint32(header[nt+24+56:])
	if imageSize < 4096 || uint64(base)+uint64(imageSize) > 1<<32 {
		return fmt.Errorf("native x86: invalid image size")
	}
	dir := binary.LittleEndian.Uint32(header[nt+24+104:])
	size := binary.LittleEndian.Uint32(header[nt+24+108:])
	if dir == 0 || size < 20 || size > imageSize || dir > imageSize-size {
		return fmt.Errorf("native x86: missing import table")
	}
	resolve := func(dll, name string) (uint32, error) {
		proc := windows.NewLazySystemDLL(dll).NewProc(name)
		if e := proc.Find(); e != nil {
			return 0, e
		}
		address, e := remoteAddress(pid, proc.Addr())
		return uint32(address), e
	}
	apis := map[string]uint32{}
	for _, name := range native386.APIs {
		address, e := resolve("kernel32.dll", name)
		if e != nil {
			return fmt.Errorf("native x86 API %s: %w", name, e)
		}
		apis[name] = address
	}
	// Console programs may not load user32. No MessageBox import exists then.
	if address, e := resolve("user32.dll", "MessageBoxW"); e == nil {
		apis["MessageBoxW"] = address
	}
	allocate := func(size int, protect uint32) (uintptr, error) {
		p, _, e := kernel.NewProc("VirtualAllocEx").Call(uintptr(process), 0, uintptr(size), windows.MEM_RESERVE|windows.MEM_COMMIT, uintptr(protect))
		if p == 0 {
			return 0, e
		}
		return p, nil
	}
	// The command line is immutable for this launcher; encode it once into a
	// small target-owned buffer so GetCommandLineA returns a stable pointer.
	var commandPointer uint32
	var commandError uint32
	wide, err := windows.UTF16FromString(syscall.EscapeArg(target))
	if err != nil {
		return err
	}
	var substituted uint32
	convert := kernel.NewProc("WideCharToMultiByte")
	n, _, _ := convert.Call(932, 0, uintptr(unsafe.Pointer(&wide[0])), uintptr(len(wide)), 0, 0, 0, uintptr(unsafe.Pointer(&substituted)))
	if n == 0 || substituted != 0 {
		commandError = 1113
	} else {
		b := make([]byte, n)
		substituted = 0
		r, _, _ := convert.Call(932, 0, uintptr(unsafe.Pointer(&wide[0])), uintptr(len(wide)), uintptr(unsafe.Pointer(&b[0])), n, 0, uintptr(unsafe.Pointer(&substituted)))
		if r == 0 || substituted != 0 {
			commandError = 1113
		} else {
			p, e := allocate(len(b), windows.PAGE_READWRITE)
			if e != nil {
				return e
			}
			if e = windows.WriteProcessMemory(process, p, &b[0], uintptr(len(b)), nil); e != nil {
				return e
			}
			commandPointer = uint32(p)
		}
	}
	hooks, err := native386.Build(apis, commandPointer, commandError)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	var code []byte
	offsets := map[string]int{}
	for _, name := range names {
		for len(code)%16 != 0 {
			code = append(code, 0xcc)
		}
		offsets[name] = len(code)
		code = append(code, hooks[name]...)
	}
	codeBase, err := allocate(len(code), windows.PAGE_READWRITE)
	if err != nil {
		return err
	}
	if err = windows.WriteProcessMemory(process, codeBase, &code[0], uintptr(len(code)), nil); err != nil {
		return err
	}
	var old uint32
	if err = windows.VirtualProtectEx(process, codeBase, uintptr(len(code)), windows.PAGE_EXECUTE_READ, &old); err != nil {
		return err
	}
	if ok, _, e := kernel.NewProc("FlushInstructionCache").Call(uintptr(process), codeBase, uintptr(len(code))); ok == 0 {
		return e
	}
	replacements := map[uint32]uint32{}
	for _, name := range names {
		dll := "kernel32.dll"
		if name == "MessageBoxA" {
			dll = "user32.dll"
		}
		original, e := resolve(dll, name)
		if e != nil {
			return e
		}
		replacements[original] = uint32(codeBase) + uint32(offsets[name])
	}
	count := 0
	for d := uint32(0); d+20 <= size; d += 20 {
		descriptor, e := read(dir+d, 20)
		if e != nil {
			return e
		}
		thunk := binary.LittleEndian.Uint32(descriptor[16:])
		if thunk == 0 {
			break
		}
		for slot := thunk; slot <= imageSize-4; slot += 4 {
			value, e := read(slot, 4)
			if e != nil {
				return e
			}
			original := binary.LittleEndian.Uint32(value)
			if original == 0 {
				break
			}
			replacement, ok := replacements[original]
			if !ok {
				continue
			}
			location := base + uintptr(slot)
			var protection uint32
			if e = windows.VirtualProtectEx(process, location, 4, windows.PAGE_READWRITE, &protection); e != nil {
				return e
			}
			binary.LittleEndian.PutUint32(value, replacement)
			writeErr := windows.WriteProcessMemory(process, location, &value[0], 4, nil)
			var ignored uint32
			restoreErr := windows.VirtualProtectEx(process, location, 4, protection, &ignored)
			if writeErr != nil {
				return writeErr
			}
			if restoreErr != nil {
				return restoreErr
			}
			count++
		}
	}
	if count == 0 {
		return fmt.Errorf("native x86: the application does not import supported locale APIs")
	}
	return nil
}
