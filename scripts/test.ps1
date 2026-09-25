param([string]$CC="gcc", [string]$CC32=$env:CC32)
$ErrorActionPreference='Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if(!$env:GOCACHE){$env:GOCACHE=Join-Path (Get-Location) '.cache/go-build'}
& (Join-Path $PSScriptRoot 'build.ps1') -EngineOnly -WithTests -CC $CC -CC32 $CC32
$saved=@{}
foreach($name in @('GOOS','GOARCH','CGO_ENABLED')){$saved[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
try {
 $env:GOOS='windows';$env:CGO_ENABLED='0'
 New-Item -ItemType Directory -Force build/tests | Out-Null
 foreach($arch in @('amd64','386')) {
  $env:GOARCH=$arch
  go test -trimpath -c -o "build/tests/launcher-tests-$arch.exe" ./internal/launcher
  if($LASTEXITCODE){throw "Test build failed: $arch"}
  & "./build/tests/launcher-tests-$arch.exe" '-test.v'
  if($LASTEXITCODE){throw "Tests failed: $arch"}
 }
 $env:GOARCH='amd64'
 go test -trimpath -c -o build/tests/integration-tests.exe ./internal/integration
 if($LASTEXITCODE){throw 'Explorer integration test build failed'}
 & ./build/tests/integration-tests.exe '-test.v'
 if($LASTEXITCODE){throw 'Explorer integration tests failed'}
 go test -trimpath -c -o build/tests/library-tests.exe ./internal/library
 if($LASTEXITCODE){throw 'Library test build failed'}
 & ./build/tests/library-tests.exe '-test.v'
 if($LASTEXITCODE){throw 'Library tests failed'}
} finally {
 foreach($name in $saved.Keys){[Environment]::SetEnvironmentVariable($name,$saved[$name],'Process')}
}
& (Join-Path $PSScriptRoot 'smoke.ps1')

& (Join-Path $PSScriptRoot 'path-smoke.ps1')
