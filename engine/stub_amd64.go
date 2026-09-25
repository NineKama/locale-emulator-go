package main

import "encoding/binary"

const peMagic = 0x20b
const importOffset = 120

// Small AMD64 leaf stubs keep Windows calling conventions and LastError intact.
// No target API call enters the Go runtime. Conversion stubs change only ECX
// (CodePage); remaining registers, stack arguments and return address are intact.
func stub(original uintptr, value uint32) []byte {
	if value != 0 {
		code := []byte{0xb8, 0, 0, 0, 0, 0xc3} // mov eax, value; ret
		binary.LittleEndian.PutUint32(code[1:], value)
		return code
	}
	code := []byte{
		0x83, 0xf9, 0x01, // cmp ecx, 1
		0x76, 0x05, // jbe replace (CP_ACP / CP_OEMCP)
		0x83, 0xf9, 0x03, // cmp ecx, 3 (CP_THREAD_ACP)
		0x75, 0x05, // jne forward
		0xb9, 0xa4, 0x03, 0, 0, // replace: mov ecx, 932
		0x48, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, // forward: mov rax, original
		0xff, 0xe0, // jmp rax
	}
	binary.LittleEndian.PutUint64(code[17:], uint64(original))
	return code
}

func ansiBridge(callback, getError, setError uintptr, argc int) []byte {
	// 32-byte shadow space, then result/error/args. Entry RSP is 8 mod 16.
	const size = 184
	code := []byte{0x48, 0x81, 0xec, size, 0, 0, 0}
	regs := [][]byte{{0x48, 0x89, 0x4c, 0x24, 48}, {0x48, 0x89, 0x54, 0x24, 56}, {0x4c, 0x89, 0x44, 0x24, 64}, {0x4c, 0x89, 0x4c, 0x24, 72}}
	for i := 0; i < argc; i++ {
		if i < 4 {
			code = append(code, regs[i]...)
		} else {
			code = append(code, 0x48, 0x8b, 0x84, 0x24)
			code = binary.LittleEndian.AppendUint32(code, uint32(size+40+(i-4)*8))
			code = append(code, 0x48, 0x89, 0x44, 0x24, byte(48+i*8))
		}
	}
	call := func(address uintptr) {
		code = append(code, 0x48, 0xb8)
		code = binary.LittleEndian.AppendUint64(code, uint64(address))
		code = append(code, 0xff, 0xd0)
	}
	code = append(code, 0x48, 0xc7, 0x44, 0x24, 32, 0, 0, 0, 0) // result
	call(getError)
	code = append(code, 0x48, 0x89, 0x44, 0x24, 40) // zero-extended DWORD
	code = append(code, 0x48, 0x8d, 0x4c, 0x24, 32)
	call(callback)
	code = append(code, 0x8b, 0x4c, 0x24, 40)
	call(setError)
	code = append(code, 0x48, 0x8b, 0x44, 0x24, 32, 0x48, 0x81, 0xc4, size, 0, 0, 0, 0xc3)
	return code
}
