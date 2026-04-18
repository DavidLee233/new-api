param(
  [switch]$Quiet
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$stateDir = Join-Path $root '.tmp'
$pidFile = Join-Path $stateDir 'local-dev-pids.json'

if (-not (Test-Path $pidFile)) {
  if (-not $Quiet) {
    Write-Host 'No local development process record was found.'
  }
  exit 0
}

$pidState = Get-Content $pidFile -Raw | ConvertFrom-Json
$stopped = @()

foreach ($procId in @($pidState.backendPid, $pidState.frontendPid)) {
  if (-not $procId) {
    continue
  }
  $process = Get-Process -Id $procId -ErrorAction SilentlyContinue
  if ($process) {
    & taskkill /PID $procId /T /F | Out-Null
    $stopped += $procId
  }
}

Remove-Item $pidFile -Force -ErrorAction SilentlyContinue

if (-not $Quiet) {
  if ($stopped.Count -eq 0) {
    Write-Host 'No running local development process was found.'
  } else {
    Write-Host ('Stopped local development processes: ' + ($stopped -join ', '))
  }
}
