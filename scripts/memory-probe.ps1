param([string]$CC32 = $env:CC32)
$ErrorActionPreference = 'Stop'
if (!$CC32) { throw 'Pass -CC32 with the x86 MinGW-compatible compiler path.' }
Push-Location (Join-Path $PSScriptRoot '..')
try {
    New-Item -ItemType Directory -Force build/tests | Out-Null
    $probe = Join-Path (Get-Location) 'build/tests/memory-probe-x86.exe'
    & $CC32 -municode -o $probe tests/memory_probe.c
    if ($LASTEXITCODE) { throw 'Memory probe compilation failed' }
    # Restrict only our synthetic probe to the address range of older x86 games.
    # Compiler linker defaults vary. No user/game executable is modified.
    $bytes = [IO.File]::ReadAllBytes($probe)
    $offset = [BitConverter]::ToInt32($bytes, 0x3c) + 22
    $flags = [BitConverter]::ToUInt16($bytes, $offset) -band 0xffdf
    [BitConverter]::GetBytes([uint16]$flags).CopyTo($bytes, $offset)
    [IO.File]::WriteAllBytes($probe, $bytes)
    & $probe (Join-Path (Get-Location) 'build/bin/locale-engine-x86.dll')
    if ($LASTEXITCODE) { throw 'Memory probe failed' }
} finally { Pop-Location }
