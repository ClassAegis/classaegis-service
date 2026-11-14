# SPDX-License-Identifier: AGPL-3.0-only
$ErrorActionPreference = 'Stop'
$uiRoot = Split-Path -Parent $PSScriptRoot
$repoRoot = Resolve-Path (Join-Path $uiRoot '..\..')
$stageRoot = Join-Path $PSScriptRoot 'stage'
$binRoot = Join-Path $stageRoot 'bin'
$confRoot = Join-Path $stageRoot 'conf'
if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $repoRoot '.gocache' }
if (-not $env:GOMODCACHE) { $env:GOMODCACHE = (go env GOMODCACHE).Trim() }

Remove-Item $stageRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $binRoot, $confRoot | Out-Null

Push-Location $repoRoot
try {
  $targets = @(
    @{ Name = 'ClassAegisCore.exe'; Package = './cmd/core'; Gui = $false },
    @{ Name = 'ClassAegisAbility.exe'; Package = './cmd/ability'; Gui = $false },
    @{ Name = 'ClassAegisLauncher.exe'; Package = './cmd/launcher'; Gui = $true },
    @{ Name = 'ClassAegisCtl.exe'; Package = './cmd/ocmctl'; Gui = $false },
    @{ Name = 'ClassAegisDebug.exe'; Package = './cmd/agent'; Gui = $false }
  )
  foreach ($target in $targets) {
    $flags = '-s -w'
    if ($target.Gui) { $flags += ' -H windowsgui' }
    & go build -trimpath -ldflags $flags -o (Join-Path $binRoot $target.Name) $target.Package
    if ($LASTEXITCODE -ne 0) { throw "Build failed: $($target.Name)" }
  }
  Copy-Item (Join-Path $repoRoot 'conf\config.example.toml') (Join-Path $confRoot 'config.example.toml')

  $licenseRoot = Join-Path $stageRoot 'licenses'
  $docsRoot = Join-Path $stageRoot 'docs'
  New-Item -ItemType Directory -Force $licenseRoot, $docsRoot | Out-Null
  Copy-Item (Join-Path $repoRoot 'LICENSE') (Join-Path $licenseRoot 'ClassAegis-LICENSE.txt')
  foreach ($name in @('AGPL-3.0-ONLY.txt','COPYRIGHT')) { Copy-Item (Join-Path $repoRoot $name) $licenseRoot }
  foreach ($name in @('SECURITY.md','CONTRIBUTING.md','CODE_OF_CONDUCT.md')) { Copy-Item (Join-Path $repoRoot $name) $docsRoot }

  $requiredFiles = @(
    'ClassAegisCore.exe',
    'ClassAegisAbility.exe',
    'ClassAegisLauncher.exe',
    'ClassAegisCtl.exe',
    'ClassAegisDebug.exe'
  )
  foreach ($name in $requiredFiles) {
    $stagedFile = Join-Path $binRoot $name
    if (-not (Test-Path $stagedFile -PathType Leaf) -or (Get-Item $stagedFile).Length -eq 0) {
      throw "Required Service component missing from installer stage: $name"
    }
  }
  if (-not (Test-Path (Join-Path $confRoot 'config.example.toml') -PathType Leaf)) {
    throw 'Required Service configuration template missing from installer stage'
  }
} finally {
  Pop-Location
}

Write-Host 'ClassAegisService components staged for packaging'
