param(
    [Parameter(Mandatory)]
    [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]*$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$output = Join-Path $project 'build/release'
New-Item -ItemType Directory -Force $output | Out-Null

# Explicit inputs keep development tools and local user data out of the archive.
$names = @('locale-emulator-go.exe', 'locale-run-x86.exe', 'locale-engine.dll', 'locale-engine-x86.dll')
$files = @($names | ForEach-Object { Join-Path $project "build/bin/$_" })
$files += Join-Path $project 'LICENSE'
$files += Join-Path $project 'AUTHORS.md'
foreach ($file in $files) {
    if (!(Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing release input: $file" }
}
$archive = Join-Path $output "Locale-Studio-$Version-windows-x64.zip"
Compress-Archive -LiteralPath $files -DestinationPath $archive -Force

# Verify the actual archive rather than assuming all inputs were included.
$zip = [IO.Compression.ZipFile]::OpenRead($archive)
try {
    $expected = @($files | ForEach-Object { Split-Path $_ -Leaf } | Sort-Object)
    $actual = @($zip.Entries.FullName | Sort-Object)
    if (Compare-Object $expected $actual) { throw 'Unexpected ZIP contents' }
} finally { $zip.Dispose() }
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  $(Split-Path $archive -Leaf)" | Set-Content "$archive.sha256" -Encoding ascii
Write-Host "Created $archive"
