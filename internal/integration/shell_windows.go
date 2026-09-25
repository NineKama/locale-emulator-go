// Package integration implements the optional per-user Explorer command.
package integration

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const menuKey = `Software\Classes\exefile\shell\LocaleStudio.Open`

// Command quotes both executable paths and never invokes cmd.exe or PowerShell.
func Command(executable string) (string, error) {
	if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\"\x00\r\n") {
		return "", fmt.Errorf("invalid Locale Studio executable path")
	}
	return `"` + executable + `" --launch "%1"`, nil
}

func Enabled(executable string) (bool, error) {
	want, err := Command(executable)
	if err != nil {
		return false, err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, menuKey+`\command`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	value, _, err := key.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return value == want, err
}

func SetEnabled(executable string, enabled bool) error {
	if !enabled {
		// Delete only our named verb; never change the default EXE association.
		for _, key := range []string{menuKey + `\command`, menuKey} {
			if err := registry.DeleteKey(registry.CURRENT_USER, key); err != nil && !errors.Is(err, registry.ErrNotExist) {
				return err
			}
		}
		return nil
	}
	command, err := Command(executable)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, menuKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	for name, value := range map[string]string{"": "Open in Locale Studio", "Icon": `"` + executable + `",0`, "MultiSelectModel": "Single"} {
		if err := key.SetStringValue(name, value); err != nil {
			return err
		}
	}
	child, _, err := registry.CreateKey(registry.CURRENT_USER, menuKey+`\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer child.Close()
	return child.SetStringValue("", command)
}

// Only an explicit launch request runs a target. Plain second launches just focus.
func Target(args []string, workingDirectory string) string {
	if len(args) != 2 || args[0] != "--launch" || strings.TrimSpace(args[1]) == "" {
		return ""
	}
	target := args[1]
	if !filepath.IsAbs(target) {
		target = filepath.Join(workingDirectory, target)
	}
	return filepath.Clean(target)
}
