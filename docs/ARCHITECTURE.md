# Architecture

## Launch sequence

1. The frontend requests a launch through `App.Launch`.
2. `launcher.Start` validates the PE machine/header and selects an engine. x86 targets are delegated to the adjacent 32-bit helper using explicit arguments and a JSON response.
3. The matching launcher creates only the selected target, sets an entry-point breakpoint, and waits for loader initialization to finish.
4. At the entry point, it restores the original instruction and RIP/EIP, suspends the primary thread, and detaches the debugger.
5. It loads the engine DLL into that process, resolving remote API addresses by module and RVA rather than assuming identical ASLR addresses.
6. It calls `InstallLocale`, checks the result and resumes the main thread. Failures terminate the newly created process so partial installation cannot leave it suspended.
7. The GUI records a successful launch in the library. Storage failures are reported separately to avoid encouraging an accidental second launch.

TLS callbacks and dependency `DllMain` routines can run before this interception point. Do not move hook installation into `DllMain`: Windows loader-lock restrictions and Go runtime initialization make that unsafe.

## Engine and calling conventions

The shared-library entry point is built with cgo. On x86, a small stdcall-to-cdecl adapter invokes the exported Go function. x64 uses the Windows x64 calling convention directly.

The engine matches resolved addresses in the main EXE's Import Address Table. PE32 slots are 4 bytes and PE32+ slots are 8 bytes. It does not patch every module in the process.

- Constant-return stubs implement the selected code page and LCID.
- Conversion stubs rewrite implicit ACP/OEM/thread code-page arguments to CP932, then tail-jump to the original Windows function. Explicit code pages are preserved.
- ANSI path callbacks decode CP932, call Unicode APIs and encode results back to CP932. Lossy filename conversion is rejected.
- Native callback frames contain the return value, LastError and original API arguments. The bridge restores LastError after returning from Go.

Stub memory is written as RW and changed to RX before publication. IAT page protections are restored after patching. Engine runtime threads, the command-line buffer and installed stubs remain alive until process exit; unloading the DLL while callbacks are reachable is not supported.

The supported ANSI bridges currently include `CreateFileA`, `GetFileAttributesA`, `GetModuleFileNameA`, `GetCommandLineA`, `GetCurrentDirectoryA`, `GetFullPathNameA` and `MessageBoxA`.

## Library

`internal/library` stores versioned JSON under the Windows user configuration directory. Mutations are serialized within one application instance. Writes use a temporary file in the same directory followed by replacement. Invalid or unsupported stored documents are not silently overwritten.

Path comparison is case-insensitive for ordinary Windows executable paths. Availability is checked when reading the list. The desktop application uses Wails single-instance locking with a stable application ID. A second launch requests that the existing window be restored and focused. The library store itself does not provide an inter-process lock for independent external writers.

## Frontend and translations

Svelte stores the active interface language and renders messages from `frontend/src/locales/`. English supplies the message schema. Identifiers, comments, CLI output and backend diagnostics remain English; localized diagnostic fragments are presentation data.

The emulated locale is currently fixed. Adding an interface translation does not add an engine profile. Future profile support must carry a validated profile through the GUI, helper and engine initialization rather than changing display text alone.
