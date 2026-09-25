package main

import "encoding/binary"

const peMagic = 0x10b
const importOffset = 104

// Win32 stdcall puts CodePage at [ESP+4]. Tail-jump to the original API so
// Windows cleans its own arguments and preserves LastError without Go callbacks.
func stub(original uintptr, value uint32) []byte {
	if value != 0 {
		code := []byte{0xb8, 0, 0, 0, 0, 0xc3}
		binary.LittleEndian.PutUint32(code[1:], value)
		return code
	}
	code := []byte{
		0x83, 0x7c, 0x24, 0x04, 0x01, // cmp dword [esp+4],1
		0x76, 0x07, // jbe replace
		0x83, 0x7c, 0x24, 0x04, 0x03, // cmp dword [esp+4],3
		0x75, 0x08, // jne forward
		0xc7, 0x44, 0x24, 0x04, 0xa4, 0x03, 0, 0, // replace: mov dword [esp+4],932
		0xb8, 0, 0, 0, 0, // forward: mov eax,original
		0xff, 0xe0, // jmp eax
	}
	binary.LittleEndian.PutUint32(code[23:], uint32(original))
	return code
}

// The Go callback writes result/LastError into a native stack frame. SetLastError
// runs after returning from Go, whose runtime may itself change thread error state.
func ansiBridge(callback, getError, setError uintptr, argc int) []byte {
	code := []byte{0x55, 0x89, 0xe5, 0x83, 0xec, byte((argc + 2) * 4)} // frame
	call := func(address uintptr) {
		code = append(code, 0xb8)
		code = binary.LittleEndian.AppendUint32(code, uint32(address))
		code = append(code, 0xff, 0xd0)
	}
	for i := 0; i < argc; i++ {
		code = append(code, 0x8b, 0x45, byte(8+i*4), 0x89, 0x44, 0x24, byte(8+i*4))
	}
	call(getError)
	code = append(code, 0x89, 0x44, 0x24, 0x04)             // frame.error
	code = append(code, 0xc7, 0x04, 0x24, 0, 0, 0, 0, 0x54) // result=0; push esp
	call(callback)
	code = append(code, 0xff, 0x74, 0x24, 0x04) // push frame.error
	call(setError)
	code = append(code, 0x8b, 0x04, 0x24, 0x89, 0xec, 0x5d, 0xc2, byte(argc*4), 0)
	return code
}
