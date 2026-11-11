param(
  [switch]$Dev
)

$ErrorActionPreference = 'Stop'
$uiRoot = Split-Path -Parent $PSScriptRoot
$tauriRoot = Join-Path $uiRoot 'src-tauri'
$userProfile = [Environment]::GetFolderPath('UserProfile')

$cargoPath = (Get-Command cargo -ErrorAction SilentlyContinue).Source
if (-not $cargoPath) {
  $cargoPath = "$userProfile\.cargo\bin\cargo.exe"
}
if (-not (Test-Path -LiteralPath $cargoPath)) {
  throw 'cargo not found; install Rust stable, MSVC Build Tools, and WebView2 Runtime.'
}
$tauriPath = (Get-Command tauri -ErrorAction SilentlyContinue).Source
if (-not $tauriPath) {
  $tauriPath = "$userProfile\.cargo\bin\cargo-tauri.exe"
}
if (-not (Test-Path -LiteralPath $tauriPath)) {
  throw 'Tauri CLI not found; run cargo install tauri-cli --version ^2.'
}

Push-Location $uiRoot
try {
  $env:VITE_OUT_DIR = 'dist-tauri'
  Push-Location $tauriRoot
  try {
    if ($Dev) {
      & $tauriPath dev
    } else {
      & $tauriPath build --config tauri.conf.json
    }
    if ($LASTEXITCODE -ne 0) { throw 'Tauri build failed' }
  } finally {
    Pop-Location
  }
} finally {
  Remove-Item Env:VITE_OUT_DIR -ErrorAction SilentlyContinue
  Pop-Location
}
