package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"locale-emulator-go/internal/integration"
	"locale-emulator-go/internal/launcher"
	"locale-emulator-go/internal/library"

	"github.com/wailsapp/wails/v2/pkg/options"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App exposes desktop actions to Wails. Presentation stays in the frontend;
// process launching and persistent library data stay in Go.
type App struct {
	ctx          context.Context
	started      chan struct{}
	library      *library.Store
	libraryError error
	requestMu    sync.Mutex
	requests     []string
}

// NewApp resolves user storage without creating files during application setup.
func NewApp() *App {
	config, err := os.UserConfigDir()
	return &App{started: make(chan struct{}), library: library.New(filepath.Join(config, "LocaleStudio", "library.json")), libraryError: err}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	close(a.started)
}

// Wait for startup if another launch arrives while the first window is loading.
// On Windows, WindowShow restores a minimised window and requests foreground focus
// without changing an already maximised window back to its normal size.
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	<-a.started
	wr.WindowShow(a.ctx)
	a.queueLaunch(data.Args, data.WorkingDirectory)
	wr.EventsEmit(a.ctx, "launch-requested")
}

// Retain requests until the frontend subscribes, including during a cold start.
func (a *App) queueLaunch(args []string, workingDirectory string) {
	if target := integration.Target(args, workingDirectory); target != "" {
		a.requestMu.Lock()
		a.requests = append(a.requests, target)
		a.requestMu.Unlock()
	}
}

func (a *App) TakeLaunchRequests() []string {
	a.requestMu.Lock()
	defer a.requestMu.Unlock()
	requests := a.requests
	a.requests = nil
	if requests == nil {
		return []string{}
	}
	return requests
}

func (a *App) ContextMenuEnabled() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	return integration.Enabled(exe)
}

func (a *App) SetContextMenuEnabled(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return integration.SetEnabled(exe, enabled)
}

// SelectExecutable opens a native file picker. Presentation strings come from
// frontend locale data; the backend keeps platform logic language-independent.
func (a *App) SelectExecutable(title, filter string) (string, error) {
	return wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{Title: title, Filters: []wr.FileFilter{{DisplayName: filter, Pattern: "*.exe"}}})
}

type LaunchResult struct {
	PID            uint32 `json:"pid"`
	Architecture   string `json:"architecture"`
	LibraryWarning string `json:"libraryWarning"`
}

// Launch records history only after a successful process launch. A storage error
// is a warning, not a launch failure: retrying would start the application twice.
func (a *App) Launch(path string) (LaunchResult, error) {
	// Launching the GUI through its own Explorer verb would forward --launch
	// back into this instance indefinitely.
	if executable, err := os.Executable(); err == nil {
		self, selfErr := os.Stat(executable)
		target, targetErr := os.Stat(path)
		if selfErr == nil && targetErr == nil && os.SameFile(self, target) {
			return LaunchResult{}, fmt.Errorf("Locale Studio cannot launch itself")
		}
	}
	result, err := launcher.Start(path, "")
	if err != nil {
		return LaunchResult{}, err
	}
	response := LaunchResult{PID: result.PID, Architecture: result.Architecture}
	if a.libraryError != nil {
		response.LibraryWarning = a.libraryError.Error()
	} else if err = a.library.Record(path, result.Architecture); err != nil {
		response.LibraryWarning = err.Error()
	}
	return response, nil
}

// GetLibrary also checks whether each executable is still present on disk.
func (a *App) GetLibrary() ([]library.Entry, error) {
	if a.libraryError != nil {
		return nil, a.libraryError
	}
	return a.library.List()
}

// RemoveFromLibrary removes metadata only, never executables or save data.
func (a *App) RemoveFromLibrary(path string) error {
	if a.libraryError != nil {
		return a.libraryError
	}
	return a.library.Remove(path)
}
