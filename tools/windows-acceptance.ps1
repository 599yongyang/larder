param(
  [string]$Binary = ".\bin\larder.exe"
)

$ErrorActionPreference = "Stop"
$client = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
$keys = @(
  "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$client",
  "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$client",
  "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$client"
)
$versions = @($keys | ForEach-Object { try { (Get-ItemProperty -Path $_ -ErrorAction Stop).pv } catch {} } | Where-Object { $_ })
if ($versions.Count -eq 0) { throw "WebView2 Evergreen runtime not found" }
Write-Host "WebView2: $($versions -join ', ')"

$fixture = Join-Path $env:TEMP 'larder-windows-acceptance'
$home = Join-Path $fixture 'home'
$data = Join-Path $fixture 'data'
$target = Join-Path $fixture 'target'
$junction = Join-Path $fixture 'junction'
Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force (Join-Path $home '.agents\skills\windows-fixture'), $data, $target | Out-Null
@"
---
name: windows-fixture
description: Windows path compatibility fixture
---
"@ | Set-Content -Encoding utf8 (Join-Path $home '.agents\skills\windows-fixture\SKILL.md')

cmd.exe /d /c mklink /J $junction $target
if (-not (Test-Path $junction -PathType Container)) { throw "junction was not created" }
$resolved = (Get-Item $junction).Target
if (-not $resolved) { throw "junction target could not be read" }
Write-Host "junction: $junction -> $resolved"

$env:LARDER_HOME = $home
$env:LARDER_DATA_DIR = $data
$process = Start-Process -FilePath $Binary -PassThru
Start-Sleep -Seconds 8
if ($process.HasExited) { throw "Larder exited during startup" }
Stop-Process -Id $process.Id -Force
Write-Host "Windows acceptance smoke passed"
