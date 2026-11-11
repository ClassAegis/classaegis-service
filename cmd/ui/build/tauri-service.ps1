param(
  [switch]$Dev
)

$ErrorActionPreference = 'Stop'
$uiRoot = Split-Path -Parent $PSScriptRoot
$tauriRoot = Join-Path $uiRoot 'src-tauri'
$buildOutput = Join-Path $env:TEMP ("classaegis-ui-" + [guid]::NewGuid().ToString('N'))
$userProfile = [Environment]::GetFolderPath('UserProfile')

$cargoPath = (Get-Command cargo -ErrorAction SilentlyContinue).Source
if (-not $cargoPath) {
  $cargoPath = "$userProfile\.cargo\bin\cargo.exe"
}
if (-not (Test-Path -LiteralPath $cargoPath)) {
  throw 'cargo not found; install Rust stable, MSVC Build Tools, and WebView2 Runtime.'
}
$cargoBin = Split-Path -Parent $cargoPath
$tauriPath = (Get-Command cargo-tauri -ErrorAction SilentlyContinue).Source
if (-not $tauriPath) { $tauriPath = Join-Path $cargoBin 'cargo-tauri.exe' }
if (-not (Test-Path -LiteralPath $tauriPath)) { $tauriPath = Join-Path $userProfile '.cargo\bin\cargo-tauri.exe' }
if (-not (Test-Path -LiteralPath $tauriPath)) {
  throw 'Tauri CLI not found; run cargo install tauri-cli --version ^2.'
}

Push-Location $uiRoot
try {
  & (Join-Path $PSScriptRoot 'stage-service.ps1')
  if ($LASTEXITCODE -ne 0) { throw 'Service component staging failed' }
  $env:VITE_OUT_DIR = $buildOutput
  Push-Location $tauriRoot
  try {
    if ($Dev) {
      & $tauriPath dev
    } else {
      $overridePath = Join-Path $env:TEMP 'classaegis-tauri-build.json'
      $override = @{ build = @{ frontendDist = $buildOutput } } | ConvertTo-Json -Compress
      Set-Content -LiteralPath $overridePath -Value $override -Encoding UTF8
      & $tauriPath build --config tauri.conf.json --config $overridePath
    }
    if ($LASTEXITCODE -ne 0) { throw 'Tauri build failed' }
  } finally {
    Pop-Location
  }
} finally {
  Remove-Item Env:VITE_OUT_DIR -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $buildOutput) {
    Remove-Item -LiteralPath $buildOutput -Recurse -Force -ErrorAction SilentlyContinue
  }
  Pop-Location
}
