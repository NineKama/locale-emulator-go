// Package native386 generates Win32 stdcall hooks. The generated code calls only
// Windows APIs; Go runs in the launcher, never in the target process.
package native386

import (
	"encoding/binary"
	"fmt"
)

type operand struct {
	kind  byte
	value int32
}

func imm(n uint32) operand    { return operand{'i', int32(n)} }
func arg(n int) operand       { return operand{'m', int32(8 + 4*n)} }
func local(n int32) operand   { return operand{'m', n} }
func address(n int32) operand { return operand{'a', n} }
func buffer(n int32) operand  { return operand{'b', n} }

type assembler struct {
	code   []byte
	labels map[string]int
	fixups map[int]string
	apis   map[string]uint32
}

func (a *assembler) emit(b ...byte)   { a.code = append(a.code, b...) }
func (a *assembler) word(n int32)     { a.code = binary.LittleEndian.AppendUint32(a.code, uint32(n)) }
func (a *assembler) mark(name string) { a.labels[name] = len(a.code) }
func (a *assembler) jump(condition byte, name string) {
	if condition == 0 {
		a.emit(0xe9)
	} else {
		a.emit(0x0f, condition)
	}
	a.fixups[len(a.code)] = name
	a.word(0)
}
func (a *assembler) load(v operand) {
	switch v.kind {
	case 'i':
		a.emit(0xb8)
		a.word(v.value)
	case 'm':
		a.emit(0x8b, 0x85)
		a.word(v.value)
	case 'a':
		a.emit(0x8d, 0x85)
		a.word(v.value)
	case 'b':
		a.emit(0x8d, 0x83)
		a.word(v.value)
	}
}
func (a *assembler) store(offset int32)         { a.emit(0x89, 0x85); a.word(offset) }
func (a *assembler) set(offset int32, n uint32) { a.load(imm(n)); a.store(offset) }
func (a *assembler) call(name string, values ...operand) {
	for i := len(values) - 1; i >= 0; i-- {
		a.load(values[i])
		a.emit(0x50)
	}
	a.emit(0xb8)
	a.word(int32(a.apis[name]))
	a.emit(0xff, 0xd0)
}
func (a *assembler) zero(label string) { a.emit(0x85, 0xc0); a.jump(0x84, label) }
func (a *assembler) finish() ([]byte, error) {
	for offset, label := range a.fixups {
		destination, ok := a.labels[label]
		if !ok {
			return nil, fmt.Errorf("unknown native label %s", label)
		}
		binary.LittleEndian.PutUint32(a.code[offset:], uint32(int32(destination-offset-4)))
	}
	return a.code, nil
}

// APIs lists the kernel32 dependencies of the native bridges.
var APIs = []string{"GetLastError", "SetLastError", "GetProcessHeap", "HeapAlloc", "HeapFree", "MultiByteToWideChar", "WideCharToMultiByte", "CreateFileW", "GetFileAttributesW", "GetModuleFileNameW", "GetCurrentDirectoryW", "GetFullPathNameW"}

// Build returns code indexed by the ANSI import it replaces. Addresses refer to
// the target's modules, resolved by module/RVA rather than local ASLR addresses.
func Build(apis map[string]uint32, commandLine, commandError uint32) (map[string][]byte, error) {
	for _, name := range APIs {
		if apis[name] == 0 {
			return nil, fmt.Errorf("missing native API %s", name)
		}
	}
	out := map[string][]byte{}
	for name, value := range map[string]uint32{"GetACP": 932, "GetOEMCP": 932, "GetUserDefaultLCID": 0x411, "GetSystemDefaultLCID": 0x411, "GetThreadLocale": 0x411} {
		code := []byte{0xb8, 0, 0, 0, 0, 0xc3}
		binary.LittleEndian.PutUint32(code[1:], value)
		out[name] = code
	}
	for _, name := range []string{"MultiByteToWideChar", "WideCharToMultiByte"} {
		code := []byte{0x83, 0x7c, 0x24, 0x04, 0x01, 0x76, 0x07, 0x83, 0x7c, 0x24, 0x04, 0x03, 0x75, 0x08, 0xc7, 0x44, 0x24, 0x04, 0xa4, 0x03, 0, 0, 0xb8, 0, 0, 0, 0, 0xff, 0xe0}
		binary.LittleEndian.PutUint32(code[23:], apis[name])
		out[name] = code
	}
	a := &assembler{apis: apis}
	if commandError != 0 {
		a.call("SetLastError", imm(commandError))
	}
	a.load(imm(commandLine))
	a.emit(0xc3)
	out["GetCommandLineA"] = a.code
	for name, argc := range map[string]int{"CreateFileA": 7, "GetFileAttributesA": 1, "GetModuleFileNameA": 3, "GetCurrentDirectoryA": 2, "GetFullPathNameA": 4, "MessageBoxA": 4} {
		if name == "MessageBoxA" && apis["MessageBoxW"] == 0 {
			continue
		}
		code, err := bridge(name, argc, apis)
		if err != nil {
			return nil, err
		}
		out[name] = code
	}
	return out, nil
}

// The only per-call allocation is a transient Windows heap buffer (192 KiB):
// two UTF-16 path buffers and one CP932 result. No large stack frame is required.
func bridge(name string, argc int, apis map[string]uint32) ([]byte, error) {
	const lastError = -16
	const result = -20
	const heap = -24
	const used = -28
	const part = -32
	const length = -36
	const prefix = -40
	a := &assembler{labels: map[string]int{}, fixups: map[int]string{}, apis: apis}
	a.emit(0x55, 0x89, 0xe5, 0x53, 0x56, 0x57, 0x83, 0xec, 0x30, 0x31, 0xdb) // frame; save nonvolatile regs; EBX = scratch
	failed := uint32(0)
	if name == "CreateFileA" || name == "GetFileAttributesA" {
		failed = 0xffffffff
	}
	a.set(result, failed)
	a.call("GetLastError")
	a.store(lastError)
	a.call("GetProcessHeap")
	a.store(heap)
	a.call("HeapAlloc", local(heap), imm(8), imm(196608))
	a.zero("oom")
	a.emit(0x89, 0xc3)
	decode := func(index int, offset int32) {
		a.load(arg(index))
		a.zero("invalid")
		a.call("MultiByteToWideChar", imm(932), imm(0), arg(index), imm(0xffffffff), buffer(offset), imm(32768))
		a.zero("apiFailure")
	}
	saveNativeError := func() { a.store(result); a.call("GetLastError"); a.store(lastError); a.jump(0, "done") }
	// Preserve caller LastError on successful path queries and conversions.
	if name == "CreateFileA" || name == "GetFileAttributesA" {
		decode(0, 0)
		a.call("SetLastError", local(lastError))
		if name == "CreateFileA" {
			a.call("CreateFileW", buffer(0), arg(1), arg(2), arg(3), arg(4), arg(5), arg(6))
		} else {
			a.call("GetFileAttributesW", buffer(0))
		}
		saveNativeError()
	} else if name == "MessageBoxA" {
		for i := 1; i <= 2; i++ {
			a.load(arg(i))
			a.zero(fmt.Sprintf("null%d", i))
			decode(i, int32((i-1)*65536))
			a.mark(fmt.Sprintf("null%d", i))
		}
		a.call("SetLastError", local(lastError))
		a.load(arg(3))
		a.emit(0x50)
		for i := 2; i >= 1; i-- {
			a.load(arg(i))
			a.zero(fmt.Sprintf("push%d", i))
			a.load(buffer(int32((i - 1) * 65536)))
			a.mark(fmt.Sprintf("push%d", i))
			a.emit(0x50)
		}
		a.load(arg(0))
		a.emit(0x50, 0xb8)
		a.word(int32(apis["MessageBoxW"]))
		a.emit(0xff, 0xd0)
		saveNativeError()
	} else {
		switch name {
		case "GetModuleFileNameA":
			a.load(arg(1))
			a.zero("small")
			a.load(arg(2))
			a.zero("small")
			a.call("GetModuleFileNameW", arg(0), buffer(65536), imm(32768))
		case "GetCurrentDirectoryA":
			a.call("GetCurrentDirectoryW", imm(32768), buffer(65536))
		case "GetFullPathNameA":
			decode(0, 0)
			a.set(part, 0)
			a.set(prefix, 0)
			a.call("GetFullPathNameW", buffer(0), imm(32768), buffer(65536), address(part))
		}
		a.zero("apiFailure")
		a.emit(0x3d)
		a.word(32768)
		a.jump(0x83, "long") // >= capacity
		a.set(used, 0)
		a.call("WideCharToMultiByte", imm(932), imm(0), buffer(65536), imm(0xffffffff), buffer(131072), imm(65536), imm(0), address(used))
		a.zero("apiFailure")
		a.store(length)
		a.load(local(used))
		a.emit(0x85, 0xc0)
		a.jump(0x85, "lossy")
		if name == "GetFullPathNameA" {
			a.load(local(part))
			a.zero("noPart")
			a.emit(0x8d, 0x93)
			a.word(65536)
			a.emit(0x29, 0xd0, 0xd1, 0xe8)
			a.zero("noPart")
			a.store(prefix)
			a.call("WideCharToMultiByte", imm(932), imm(0), buffer(65536), local(prefix), imm(0), imm(0), imm(0), imm(0))
			a.zero("apiFailure")
			a.store(prefix)
			a.mark("noPart")
			a.load(arg(3))
			a.zero("partCleared")
			a.emit(0xc7, 0x00, 0, 0, 0, 0)
			a.mark("partCleared")
		}
		sizeIndex, destIndex := 0, 1
		if name == "GetModuleFileNameA" {
			sizeIndex = 2
		} else if name == "GetFullPathNameA" {
			sizeIndex = 1
			destIndex = 2
		}
		a.load(local(length))
		a.emit(0x3b, 0x85)
		a.word(arg(sizeIndex).value)
		a.jump(0x86, "fits") // count <= size
		if name == "GetModuleFileNameA" {
			a.load(arg(sizeIndex))
			a.store(length)
			a.store(result)
			a.set(lastError, 122)
			a.jump(0, "copy")
		} else {
			a.store(result)
			a.jump(0, "done")
		}
		a.mark("fits")
		a.emit(0x48)
		a.store(result)
		a.mark("copy")
		a.load(arg(destIndex))
		a.zero("invalid")
		a.emit(0x89, 0xc7) // EDI destination
		a.load(buffer(131072))
		a.emit(0x89, 0xc6)
		a.load(local(length))
		a.emit(0x89, 0xc1, 0xfc, 0xf3, 0xa4) // cld; rep movsb
		if name == "GetModuleFileNameA" {
			a.emit(0xc6, 0x47, 0xff, 0)
		}
		if name == "GetFullPathNameA" {
			a.load(arg(3))
			a.zero("done")
			a.emit(0x89, 0xc2)
			a.load(local(part))
			a.zero("done")
			a.load(arg(2))
			a.emit(0x03, 0x85)
			a.word(prefix)
			a.emit(0x89, 0x02)
		}
		a.jump(0, "done")
	}
	a.mark("apiFailure")
	a.call("GetLastError")
	a.store(lastError)
	a.set(result, failed)
	a.jump(0, "done")
	for label, code := range map[string]uint32{"invalid": 87, "small": 122, "long": 206, "lossy": 1113, "oom": 8} {
		a.mark(label)
		a.set(lastError, code)
		a.set(result, failed)
		a.jump(0, "done")
	}
	a.mark("done")
	a.emit(0x85, 0xdb)
	a.jump(0x84, "return")
	// HeapFree may change LastError; restore it only after cleanup finishes.
	a.call("HeapFree", local(heap), imm(0), buffer(0))
	a.mark("return")
	a.call("SetLastError", local(lastError))
	a.load(local(result))
	a.emit(0x8d, 0x65, 0xf4, 0x5f, 0x5e, 0x5b, 0x5d, 0xc2, byte(argc*4), 0)
	return a.finish()
}
