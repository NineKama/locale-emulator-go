package launcher

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/windows"
)

const (
	debugPadding           = 0
	debugEntryOffset       = 28
	exceptionAddressOffset = 12
	contextSize            = 716
	contextFlagsOffset     = 0
	contextIPOffset        = 184
	contextControl         = 0x10001
)

// Go's c-shared export is cdecl; Windows LPTHREAD_START_ROUTINE is stdcall.
// Adapt the single argument and clean both stack frames, preserving EAX.
func installThreadEntry(process windows.Handle, address uintptr) (uintptr, func(), error) {
	code := []byte{
		0xff, 0x74, 0x24, 0x04, // push dword [esp+4]
		0xb8, 0, 0, 0, 0, // mov eax, InstallLocale
		0xff, 0xd0, // call eax
		0x83, 0xc4, 0x04, // add esp,4 (cdecl argument)
		0xc2, 0x04, 0x00, // ret 4 (stdcall argument)
	}
	binary.LittleEndian.PutUint32(code[5:], uint32(address))
	mem, _, e := kernel.NewProc("VirtualAllocEx").Call(uintptr(process), 0, uintptr(len(code)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if mem == 0 {
		return 0, nil, fmt.Errorf("allocate x86 adapter: %w", e)
	}
	cleanup := func() { kernel.NewProc("VirtualFreeEx").Call(uintptr(process), mem, 0, windows.MEM_RELEASE) }
	if e = windows.WriteProcessMemory(process, mem, &code[0], uintptr(len(code)), nil); e != nil {
		cleanup()
		return 0, nil, e
	}
	var old uint32
	if e = windows.VirtualProtectEx(process, mem, uintptr(len(code)), windows.PAGE_EXECUTE_READ, &old); e != nil {
		cleanup()
		return 0, nil, e
	}
	kernel.NewProc("FlushInstructionCache").Call(uintptr(process), mem, uintptr(len(code)))
	return mem, cleanup, nil
}
