package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type Result struct {
	PID          uint32 `json:"pid"`
	Architecture string `json:"architecture"`
}

// Start chooses a matching engine/helper. An empty dll selects the bundled DLL.
func Start(target, dll string) (Result, error) {
	arch, e := Architecture(target)
	if e != nil {
		return Result{}, e
	}
	exe, e := os.Executable()
	if e != nil {
		return Result{}, e
	}
	dir := filepath.Dir(exe)
	// Default x86 launches use native code generated in the helper, avoiding a
	// Go runtime inside the game. An explicit DLL override remains diagnostic-only.
	native := dll == "" && arch == "386"
	if dll == "" {
		name := "locale-engine.dll"
		if arch == "386" {
			name = "locale-engine-x86.dll"
		}
		dll = filepath.Join(dir, name)
	}
	dll, e = filepath.Abs(dll)
	if e != nil {
		return Result{}, e
	}
	if arch == runtime.GOARCH {
		pid, e := launch(target, dll, native)
		return Result{pid, arch}, e
	}
	if runtime.GOARCH != "amd64" || arch != "386" {
		return Result{}, fmt.Errorf("This helper only runs applications of the same architecture; use locale-run.exe to select x86/x64 automatically")
	}
	helper := filepath.Join(dir, "locale-run-x86.exe")
	if _, e = os.Stat(helper); e != nil {
		return Result{}, fmt.Errorf("x86 helper missing beside the application: %w", e)
	}
	args := []string{"-json", target}
	if !native {
		args = []string{"-json", "-engine", dll, target}
	}
	cmd := exec.Command(helper, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, e := cmd.Output()
	if e != nil {
		if exit, ok := e.(*exec.ExitError); ok {
			return Result{}, fmt.Errorf("helper x86: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return Result{}, fmt.Errorf("Start x86 helper: %w", e)
	}
	var result Result
	if e = json.Unmarshal(out, &result); e != nil {
		return Result{}, fmt.Errorf("Invalid x86 helper response: %w", e)
	}
	if result.PID == 0 || result.Architecture != arch {
		return Result{}, fmt.Errorf("The x86 helper returned an invalid result")
	}
	return result, nil
}
