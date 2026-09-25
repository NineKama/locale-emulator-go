package launcher

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, machine uint16, is32, managed, dll bool) string {
	t.Helper()
	var optional any
	size := uint16(binary.Size(pe.OptionalHeader64{}))
	if is32 {
		h := pe.OptionalHeader32{Magic: 0x10b, NumberOfRvaAndSizes: 16}
		if managed {
			h.DataDirectory[14].VirtualAddress = 0x1000
		}
		optional = h
		size = uint16(binary.Size(h))
	} else {
		h := pe.OptionalHeader64{Magic: 0x20b, NumberOfRvaAndSizes: 16}
		if managed {
			h.DataDirectory[14].VirtualAddress = 0x1000
		}
		optional = h
	}
	data := make([]byte, 64)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[60:], 64)
	b := bytes.NewBuffer(data)
	b.WriteString("PE\x00\x00")
	flags := uint16(pe.IMAGE_FILE_EXECUTABLE_IMAGE)
	if dll {
		flags |= pe.IMAGE_FILE_DLL
	}
	if e := binary.Write(b, binary.LittleEndian, pe.FileHeader{Machine: machine, SizeOfOptionalHeader: size, Characteristics: flags}); e != nil {
		t.Fatal(e)
	}
	if e := binary.Write(b, binary.LittleEndian, optional); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "fixture.exe")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestArchitecture(t *testing.T) {
	tests := []struct {
		name               string
		machine            uint16
		is32, managed, dll bool
		arch, want         string
	}{
		{"x64", pe.IMAGE_FILE_MACHINE_AMD64, false, false, false, "amd64", ""},
		{"x86", pe.IMAGE_FILE_MACHINE_I386, true, false, false, "386", ""},
		{"arm64", pe.IMAGE_FILE_MACHINE_ARM64, false, false, false, "", "ARM64"},
		{"dll64", pe.IMAGE_FILE_MACHINE_AMD64, false, false, true, "", "DLL"},
		{"dll32", pe.IMAGE_FILE_MACHINE_I386, true, false, true, "", "DLL"},
		{"managed64", pe.IMAGE_FILE_MACHINE_AMD64, false, true, false, "", ".NET"},
		{"managed32", pe.IMAGE_FILE_MACHINE_I386, true, true, false, "", ".NET"},
		{"mismatched32", pe.IMAGE_FILE_MACHINE_I386, false, false, false, "", "PE32"},
		{"mismatched64", pe.IMAGE_FILE_MACHINE_AMD64, true, false, false, "", "PE64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arch, e := Architecture(fixture(t, tt.machine, tt.is32, tt.managed, tt.dll))
			if tt.want != "" {
				if e == nil || !strings.Contains(e.Error(), tt.want) {
					t.Fatalf("got %q %v; want error %q", arch, e, tt.want)
				}
				return
			}
			if e != nil || arch != tt.arch {
				t.Fatalf("got %q %v; want %q", arch, e, tt.arch)
			}
		})
	}
}
func TestInvalidPaths(t *testing.T) {
	for _, path := range []string{"relative.exe", filepath.Join(t.TempDir(), "missing.exe")} {
		if Validate(path) == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	path := filepath.Join(t.TempDir(), "broken.exe")
	if e := os.WriteFile(path, []byte("not PE"), 0600); e != nil {
		t.Fatal(e)
	}
	if Validate(path) == nil {
		t.Fatal("accepted malformed PE")
	}
}
