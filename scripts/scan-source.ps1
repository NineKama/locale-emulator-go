param([string]$Gitleaks = 'gitleaks')

# Audit exactly the tracked and unignored source files that could be published.
# Generated outputs and local application history are deliberately excluded.
$ErrorActionPreference = 'Stop'
$project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$scanner = (Get-Command $Gitleaks -ErrorAction Stop).Source
Push-Location $project
$snapshot = Join-Path $project ('.cache/source-audit/' + [guid]::NewGuid())
try {
    $files = @(git -c core.quotepath=false ls-files --cached --others --exclude-standard | Sort-Object -Unique)
    if ($LASTEXITCODE -or !$files.Count) { throw 'Cannot enumerate Git source files.' }
    New-Item -ItemType Directory -Force $snapshot, 'build/reports' | Out-Null
    $privacyFindings = @()
    $rules = @{
        'Personal home directory' = '(?i)[a-z]:[\\/]+Users[\\/]+[^\s"<>]+'
        'Email address' = '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'
    }
    foreach ($file in $files) {
        $source = [IO.Path]::GetFullPath((Join-Path $project $file))
        if (!$source.StartsWith($project + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Source path outside project.'
        }
        # Reject links so an audit cannot silently copy files from another folder.
        $item = Get-Item -LiteralPath $source
        $ancestor = $item
        while ($ancestor.FullName -ne $project) {
            if ($ancestor.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Linked source path: $file" }
            if ($ancestor -is [IO.FileInfo]) { $ancestor = $ancestor.Directory }
            else { $ancestor = $ancestor.Parent }
        }
        $destination = Join-Path $snapshot $file
        New-Item -ItemType Directory -Force (Split-Path $destination) | Out-Null
        Copy-Item -LiteralPath $source -Destination $destination
        if ([IO.Path]::GetExtension($file) -in @('.png', '.ico')) { continue }
        $content = [IO.File]::ReadAllText($source)
        foreach ($rule in $rules.GetEnumerator()) {
            if ($content -match $rule.Value) {
                # Never echo potentially sensitive matched values into logs.
                $privacyFindings += [pscustomobject]@{ file = $file; rule = $rule.Key }
            }
        }
    }
    $report = Join-Path $project 'build/reports/secrets.json'
    & $scanner dir $snapshot --no-banner --no-color --redact --report-format json --report-path $report
    $scanExitCode = $LASTEXITCODE
    ConvertTo-Json -InputObject @($privacyFindings) | Set-Content 'build/reports/privacy.json' -Encoding utf8
    Write-Host "Audited $($files.Count) source files; privacy findings: $($privacyFindings.Count)."
    if ($privacyFindings.Count) { $privacyFindings | Format-Table; throw 'Review privacy findings before publishing.' }
    if ($scanExitCode) { throw "Gitleaks failed or found secrets (exit $scanExitCode). See the redacted report." }
} finally {
    $resolved = [IO.Path]::GetFullPath($snapshot)
    $auditRoot = [IO.Path]::GetFullPath((Join-Path $project '.cache/source-audit')) + [IO.Path]::DirectorySeparatorChar
    if (!$resolved.StartsWith($auditRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe audit cleanup path.' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
    Pop-Location
}
