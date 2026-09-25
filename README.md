# Locale Studio — Windows Locale Emulator

**Locale Studio** is a Windows locale emulator that launches x86 and x64 games and applications with per-app locale settings, without changing your system locale. It includes a persistent application library and an English/Vietnamese interface.

**Author: Dat Diep** · [Tiếng Việt](docs/README.vi.md) · [Architecture](docs/ARCHITECTURE.md)

## Features

- Keep one desktop instance; opening the app again brings the existing window forward.
- Launch native **x86 and x64** applications from one interface.
- Automatically select the matching engine and helper.
- Save successfully launched applications to a searchable library, sorted by recent use.
- Launch library entries again without selecting their executable each time.
- Switch between **English and Vietnamese**; the selection persists locally.
- Handle selected ANSI file/path APIs using Unicode Windows APIs internally.
- Keep the system locale and the original executable unchanged.

The current emulation profile is **ja-JP / CP932 / LCID 0x0411**. Additional locale profiles are not implemented yet. UI language selection and the emulated application locale are separate settings.

## Requirements

To run:

- Windows 10/11 **x64**.
- Microsoft Edge WebView2 Runtime.
- A native Windows x86 or x64 target executable.

To build:

- Go **1.25 or newer**, as declared in `go.mod`.
- Wails CLI **v2.14.0**.
- Node.js **22.12 or newer** and npm, compatible with Vite 7.
- MinGW-compatible C compilers for **both x64 and x86**. GCC/MinGW-w64 or LLVM-MinGW can be used.

The engine is implemented in Go, with generated native bridge stubs. Building its DLL exports requires **cgo and a C compiler**. The C files in `tests/` are independent native test programs, not the engine implementation.

## Build

Run from the repository root in PowerShell:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0

# If the compilers are on PATH:
./scripts/build.ps1 -CC x86_64-w64-mingw32-gcc -CC32 i686-w64-mingw32-gcc

# LLVM-MinGW is also supported:
./scripts/build.ps1 -CC x86_64-w64-mingw32-clang -CC32 i686-w64-mingw32-clang
```

Pass a full compiler path when it is not on PATH. `CC32` can also be supplied as an environment variable. The script uses `npm ci` and the committed lockfile for frontend dependencies. Compiler installation and downloads are not performed by the build script.

The runtime package contains exactly four files in `build/bin/`:

```text
locale-emulator-go.exe    Desktop application
locale-run-x86.exe        32-bit helper, launched automatically
locale-engine.dll        64-bit engine
locale-engine-x86.dll    32-bit engine
```

Keep all four files together. Open **`locale-emulator-go.exe`**, select an executable, and click **Launch**. The helper is not a second application users need to open manually.

Build engine/helper components without rebuilding the GUI:

```powershell
./scripts/build.ps1 -EngineOnly -CC32 i686-w64-mingw32-gcc
```

Build optional CLI and test tools separately:

```powershell
./scripts/build.ps1 -EngineOnly -WithTests -CC32 i686-w64-mingw32-gcc
./build/tests/locale-run.exe 'C:\Apps\example.exe'
```

Development tools go to `build/tests/`; generated DLL headers go to `build/generated/`. Neither directory belongs in the source repository or the runtime package.


## Explorer context menu

Open Locale Studio and click **Enable context menu** under **Explorer integration**.
Then right-click a native `.exe` and choose **Open in Locale Studio** to launch it
through the engine automatically. On Windows 11, the entry may be under **Show
more options**. Requests go to the existing window when Locale Studio is running;
otherwise it opens first. Successful launches also enter the application library.

Registration applies only to the current Windows user and does not require admin
rights or replace the normal double-click action. Use **Disable context menu**
before deleting the portable app. After moving the app folder, enable the menu
again from the new location. Keep all four runtime files together.

The equivalent command is `locale-emulator-go.exe --launch "C:\Apps\game.exe"`.
The integration accepts one EXE at a time; shortcuts and extra game arguments are
not supported by this command.

## Library and local data

Successful GUI launches are recorded in `%APPDATA%\LocaleStudio\library.json`. The library stores executable paths, names, architecture, last-launch time and launch count. Removing an entry **does not delete the application or its save files**.

Missing or moved executables are marked unavailable. Refresh the list or select the new location. Runs made before the library feature existed, or through the test CLI, are not automatically added.

The interface language is saved in the WebView's local storage. The application does not upload your library. Do not include personal library files, WebView profiles or diagnostic logs when sharing the source.

## Compatibility limits

This is an experimental compatibility tool, not a complete Windows locale replacement.

- The UI runs on x64 Windows; the helper supports x86 applications on that system.
- ARM64 and .NET targets are rejected.
- Hooks cover normal imports in the **main EXE**, not all loaded DLLs or dynamically resolved APIs.
- Delay imports, child processes, registry emulation, fonts/GDI and time zones are not covered.
- TLS callbacks and dependency initialization may execute before hooks are installed.
- Applications requiring elevation or restricting debugging/dynamic code may not work.
- ANSI paths must be representable in CP932; other characters can still be unsupported.
- A successful launch notification confirms hook installation, not full game compatibility.

The launcher does not bypass DRM, anti-cheat, operating-system protections or application access controls.

## Tests

```powershell
# Builds both architectures and runs unit + native integration tests.
./scripts/test.ps1 -CC32 i686-w64-mingw32-gcc

# Frontend checks, after a full build has generated Wails bindings:
npm --prefix frontend run check

# Re-run existing native test binaries:
./scripts/smoke.ps1
./scripts/path-smoke.ps1
```

Tests cover PE architecture validation, library persistence and deduplication, non-destructive library removal, malformed stored data, CP932 conversion, unchanged explicit UTF-8, Windows error propagation, x86 calling conventions, and synthetic non-ASCII executable paths.

The direct native baseline may print `FAIL` when Windows is not configured for Japanese. That is the comparison case; emulated runs must print `PASS`.

## Source layout

| Path | Responsibility |
| --- | --- |
| `app.go` | Wails bindings and library integration |
| `internal/launcher/` | PE validation, architecture routing and process startup |
| `internal/library/` | Persistent application library |
| `engine/` | DLL entry point, native stubs and ANSI API bridges |
| `cmd/locale-run/` | CLI used for diagnostics and the x86 helper |
| `frontend/src/` | Svelte interface and English application logic |
| `frontend/src/locales/` | Interface translations and localized diagnostic data |
| `tests/` | Native C fixtures |
| `scripts/` | Build, test and source-audit commands |

Identifiers, comments and backend diagnostics are English. Translations are data files. Generated Wails bindings are ignored and recreated during builds.

## Sharing this repository

See [Publishing and privacy](docs/PUBLISHING.md) for the source audit, generated-file exclusions and release packaging. There are no game files or personal library records in the intended source set.

Licensed under the [MIT License](LICENSE). Author attribution is recorded in [AUTHORS.md](AUTHORS.md); dependencies retain their own licenses and notices.

Portable releases include generated third-party license texts. See
[Third-party notices](THIRD-PARTY.md) for their scope and update procedure.

For automated Windows ZIP releases, see [Automated ZIP releases](docs/PUBLISHING.md#automated-zip-releases).
