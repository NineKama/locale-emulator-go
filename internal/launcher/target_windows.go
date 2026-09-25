package launcher

import (
	"debug/pe"
	"fmt"
	"path/filepath"
	"strings"
)

func Validate(path string) error { _, e := Architecture(path); return e }

// Architecture checks the machine and PE header before creating any process.
// Managed executables are excluded because this engine targets native imports.
func Architecture(path string) (string, error) {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", fmt.Errorf("Select an absolute path to an x86 or x64 .exe file")
	}
	f, e := pe.Open(path)
	if e != nil {
		return "", fmt.Errorf("Read PE: %w", e)
	}
	defer f.Close()
	if f.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		return "", fmt.Errorf("Select an executable, not a DLL")
	}
	var arch string
	var managed uint32
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		h, ok := f.OptionalHeader.(*pe.OptionalHeader64)
		if !ok {
			return "", fmt.Errorf("Invalid PE64 image")
		}
		arch = "amd64"
		managed = h.DataDirectory[14].VirtualAddress
	case pe.IMAGE_FILE_MACHINE_I386:
		h, ok := f.OptionalHeader.(*pe.OptionalHeader32)
		if !ok {
			return "", fmt.Errorf("Invalid PE32 image")
		}
		arch = "386"
		managed = h.DataDirectory[14].VirtualAddress
	default:
		return "", fmt.Errorf("Only x86 / x64 applications are supported; ARM64 and other architectures are not supported yet")
	}
	if managed != 0 {
		return "", fmt.Errorf(".NET applications are not supported in this MVP")
	}
	return arch, nil
}
