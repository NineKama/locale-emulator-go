param(
 [string]$CC = "gcc",
 [string]$CC32 = $env:CC32,
 [switch]$EngineOnly,
 [switch]$WithTests
)
# Build a portable Windows package. Test binaries are opt-in and never enter bin.
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if (!$env:GOCACHE) { $env:GOCACHE=Join-Path (Get-Location) '.cache/go-build' }
if (!$CC32) {
 $candidates=@(
  'i686-w64-mingw32-gcc',
  'i686-w64-mingw32-clang'
 )
 foreach($candidate in $candidates){
  $command=Get-Command $candidate -ErrorAction SilentlyContinue
  if($command){$CC32=$command.Source;break}
 }
}
if (!$CC32) { throw 'Missing x86 C compiler. Pass -CC32 path/to/i686-w64-mingw32-gcc.exe or i686-w64-mingw32-clang.exe.' }
$saved=@{}
foreach($name in @('GOOS','GOARCH','CGO_ENABLED','CC')){$saved[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
try {
 $env:GOOS='windows'
 # cgo is required only for exported DLL entry points; the engine logic is Go.
 $env:CGO_ENABLED='1'
 New-Item -ItemType Directory -Force build/bin,build/generated | Out-Null
 if ($WithTests) { New-Item -ItemType Directory -Force build/tests | Out-Null }
 foreach($target in @(
  @{Arch='amd64'; Compiler=$CC; Suffix=''},
  @{Arch='386'; Compiler=$CC32; Suffix='-x86'}
 )) {
  $env:GOARCH=$target.Arch
  $env:CC=$target.Compiler
  $suffix=$target.Suffix
  Write-Host "Building $($target.Arch)..."
  go build -trimpath -ldflags "-s -w" -buildmode=c-shared -o "build/generated/locale-engine$suffix.dll" ./engine
  if ($LASTEXITCODE) { throw "Engine $($target.Arch) build failed" }
  Copy-Item -LiteralPath "build/generated/locale-engine$suffix.dll" -Destination "build/bin/locale-engine$suffix.dll" -Force
  if ($suffix -eq '-x86') {
   go build -trimpath -ldflags "-s -w" -o build/bin/locale-run-x86.exe ./cmd/locale-run
   if ($LASTEXITCODE) { throw 'x86 helper build failed' }
  }
  if ($WithTests) {
   go build -trimpath -ldflags "-s -w" -o "build/tests/locale-run$suffix.exe" ./cmd/locale-run
   if ($LASTEXITCODE) { throw "Test launcher $($target.Arch) build failed" }
   Copy-Item -LiteralPath "build/generated/locale-engine$suffix.dll" -Destination "build/tests/locale-engine$suffix.dll" -Force
   & $target.Compiler -O0 -o "build/tests/locale-probe$suffix.exe" tests/probe.c
   if ($LASTEXITCODE) { throw "Probe $($target.Arch) build failed" }
   & $target.Compiler -O0 -o "build/tests/path-probe$suffix.exe" tests/path_probe.c
   if ($LASTEXITCODE) { throw "Path probe $($target.Arch) build failed" }
  }
 }
 # Keep all four runtime files together; the GUI locates its helper beside itself.
 if (!$EngineOnly) {
  $env:GOARCH='amd64';$env:CC=$CC
  wails build -trimpath -platform windows/amd64
  if ($LASTEXITCODE) { throw 'Wails build failed' }
 }
} finally {
 foreach($name in $saved.Keys){[Environment]::SetEnvironmentVariable($name,$saved[$name],'Process')}
}
