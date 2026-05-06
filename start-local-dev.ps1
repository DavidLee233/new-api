$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$stateDir = Join-Path $root '.tmp'
$logDir = Join-Path $root 'logs'
$dataDir = Join-Path $root 'data'
$pidFile = Join-Path $stateDir 'local-dev-pids.json'
$stopScript = Join-Path $root 'stop-local-dev.ps1'
$backendBuildLog = Join-Path $logDir 'backend-build.log'
$goCacheDir = Join-Path $stateDir 'gocache'
$goTmpDir = Join-Path $stateDir 'gotmp'

function Quote-Argument {
  param([string]$Value)

  if ($null -eq $Value -or $Value -eq '') {
    return '""'
  }
  if ($Value -match '[\s"]') {
    return '"' + ($Value -replace '"', '\"') + '"'
  }
  return $Value
}

function Start-DetachedProcess {
  param(
    [string]$FilePath,
    [string[]]$Arguments,
    [string]$WorkingDirectory
  )

  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $FilePath
  $psi.WorkingDirectory = $WorkingDirectory
  $psi.UseShellExecute = $false
  $psi.CreateNoWindow = $true
  $psi.Arguments = [string]::Join(' ', ($Arguments | ForEach-Object { Quote-Argument $_ }))

  $process = New-Object System.Diagnostics.Process
  $process.StartInfo = $psi
  [void]$process.Start()
  return $process
}

foreach ($dir in @($stateDir, $logDir, $dataDir, $goCacheDir, $goTmpDir)) {
  if (-not (Test-Path $dir)) {
    New-Item -ItemType Directory -Path $dir | Out-Null
  }
}

if (Test-Path $pidFile) {
  & $stopScript -Quiet
}

$goCommand = (Get-Command go -ErrorAction Stop).Source
$nodeCommand = (Get-Command node -ErrorAction Stop).Source
$backendBinary = Join-Path $stateDir 'new-api-dev.exe'
$env:GOCACHE = $goCacheDir
$env:GOTMPDIR = $goTmpDir

$quotedGoCommand = '"' + ($goCommand -replace '"', '\"') + '"'
$quotedBackendBinary = '"' + ($backendBinary -replace '"', '\"') + '"'
$quotedBackendBuildLog = '"' + ($backendBuildLog -replace '"', '\"') + '"'
cmd.exe /c "$quotedGoCommand build -o $quotedBackendBinary . > $quotedBackendBuildLog 2>&1"
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $backendBinary)) {
  Write-Host 'Backend build failed. Recent log:' -ForegroundColor Red
  if (Test-Path $backendBuildLog) {
    Get-Content $backendBuildLog -Tail 40
  }
  throw 'Backend build failed.'
}

$backendProcess = Start-DetachedProcess `
  -FilePath $backendBinary `
  -Arguments @('--log-dir', './logs') `
  -WorkingDirectory $root

$bunCommand = $null
$bunInstalled = Get-Command bun -ErrorAction SilentlyContinue
if ($bunInstalled) {
  $bunCommand = $bunInstalled.Source
} else {
  $bunCandidate = Join-Path $env:USERPROFILE '.bun\bin\bun.exe'
  if (Test-Path $bunCandidate) {
    $bunCommand = $bunCandidate
  }
}

$frontendWorkingDirectory = Join-Path $root 'web'
if ($bunCommand) {
  $frontendTool = 'bun'
  $frontendProcess = Start-DetachedProcess `
    -FilePath $bunCommand `
    -Arguments @('run', 'dev', '--', '--host', '127.0.0.1') `
    -WorkingDirectory $frontendWorkingDirectory
} else {
  $frontendTool = 'node'
  $viteCli = Join-Path $frontendWorkingDirectory 'node_modules\vite\bin\vite.js'
  if (-not (Test-Path $viteCli)) {
    throw 'Vite CLI was not found. Run frontend dependency installation first.'
  }
  $frontendProcess = Start-DetachedProcess `
    -FilePath $nodeCommand `
    -Arguments @($viteCli, '--host', '127.0.0.1') `
    -WorkingDirectory $frontendWorkingDirectory
}

Start-Sleep -Seconds 8

$failed = @()
if ($backendProcess.HasExited) {
  $failed += 'backend'
}
if ($frontendProcess.HasExited) {
  $failed += 'frontend'
}

if ($failed.Count -gt 0) {
  & $stopScript -Quiet
  if ($failed -contains 'backend') {
    Write-Host 'Backend failed to stay up. Check logs in .\logs.' -ForegroundColor Red
  }
  if ($failed -contains 'frontend') {
    Write-Host 'Frontend failed to stay up. Check the Vite dependencies under .\web\node_modules.' -ForegroundColor Red
  }
  throw ('Startup failed for: ' + ($failed -join ', '))
}

$pidState = @{
  backendPid   = $backendProcess.Id
  frontendPid  = $frontendProcess.Id
  frontendTool = $frontendTool
  startedAt    = (Get-Date).ToString('s')
} | ConvertTo-Json

Set-Content -Path $pidFile -Value $pidState -Encoding utf8

Write-Host ''
Write-Host 'Local development services are running.' -ForegroundColor Green
Write-Host 'Backend : http://127.0.0.1:3000'
Write-Host 'Frontend: http://127.0.0.1:5173'
Write-Host 'Admin   : root / Admin@123456'
Write-Host 'DB      : SQLite file .\data\new-api.db (no database password)'
Write-Host ''
Write-Host "Logs: $logDir"
Write-Host 'Stop : .\stop-local-dev.ps1 or stop-local-dev.cmd'
