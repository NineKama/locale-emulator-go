$ErrorActionPreference='Stop'
$project=Split-Path $PSScriptRoot -Parent
$bin=Join-Path $project 'build/tests'
# These code points spell a synthetic Japanese test directory name.
$unicodeName=-join ([char[]](0x65e5,0x672c,0x8a9e))
$folder=Join-Path $bin ($unicodeName + ' test')
New-Item -ItemType Directory -Force $folder | Out-Null
foreach($suffix in @('','-x86')){
 $bits=if($suffix){32}else{64}
 $target=Join-Path $folder ($unicodeName + " sample $bits.exe")
 Copy-Item -LiteralPath (Join-Path $bin "path-probe$suffix.exe") -Destination $target -Force
 $result=Join-Path $folder 'path-result.txt'
 if(Test-Path -LiteralPath $result){Remove-Item -LiteralPath $result}
 & (Join-Path $bin 'locale-run.exe') '-json' $target
 if($LASTEXITCODE){throw 'Path probe launch failed'}
 $deadline=(Get-Date).AddSeconds(15);$text=''
 while((Get-Date) -lt $deadline){
  try{$text=Get-Content -LiteralPath $result -Raw -ErrorAction Stop}catch{}
  if($text -match '^PASS|^FAIL'){break}
  Start-Sleep -Milliseconds 100
 }
 if($text -notmatch "^PASS bits=$bits "){throw "Path probe failed: $text"}
 Write-Host $text
}
