$ErrorActionPreference='Stop'
$project=Split-Path $PSScriptRoot -Parent
$bin=Join-Path $project 'build/tests'
$result=Join-Path $bin 'probe-result.txt'
foreach($target in @(
 @{Bits=64;Suffix='';Runner='locale-run.exe';Label='x64'},
 @{Bits=32;Suffix='-x86';Runner='locale-run.exe';Label='x86 through x64 dispatcher'},
 @{Bits=32;Suffix='-x86';Runner='locale-run-x86.exe';Label='x86 helper directly'}
)) {
 $probe=Join-Path $bin "locale-probe$($target.Suffix).exe"
 $launcher=Join-Path $bin $target.Runner
 $baseline=Start-Process -FilePath $probe -WorkingDirectory $bin -WindowStyle Hidden -PassThru -Wait
 Write-Host "Native $($target.Bits)-bit baseline: $(Get-Content -LiteralPath $result -Raw)"
 # Include a non-ASCII filename without referring to a real application.
 $copy=Join-Path $bin ("caf" + [char]0x00e9 + " locale $($target.Bits).exe")
 Copy-Item -LiteralPath $probe -Destination $copy -Force
 try {
  for($i=0;$i -lt 3;$i++) {
   if(Test-Path -LiteralPath $result){Remove-Item -LiteralPath $result}
   $raw=& $launcher '-json' $copy
   if($LASTEXITCODE){throw "Launcher failed: $($target.Label)"}
   $launched=$raw | ConvertFrom-Json
   $expectedArch=if($target.Bits -eq 32){'386'}else{'amd64'}
   if(!$launched.pid -or $launched.architecture -ne $expectedArch){throw "Wrong architecture response: $raw"}
   $deadline=(Get-Date).AddSeconds(10)
   $text=''
   $pattern="^PASS bits=$($target.Bits) .*error=87"
   while((Get-Date) -lt $deadline){
    try{$text=Get-Content -LiteralPath $result -Raw -ErrorAction Stop}catch{}
    if($text -match $pattern){break}
    Start-Sleep -Milliseconds 100
   }
   if($text -notmatch $pattern){throw "Probe failed ($($target.Label)): $text"}
   Write-Host "$($target.Label), run $($i+1): $text"
  }
 } finally { Remove-Item -LiteralPath $copy -ErrorAction SilentlyContinue }
}
