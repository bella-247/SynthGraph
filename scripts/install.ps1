#!/usr/bin/env pwsh
# SynthGraph Installer — Windows (PowerShell)
# Usage: irm https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.ps1 | iex
# Or:    .\scripts\install.ps1

$Repo = "bella-247/SynthGraph"
$Version = "latest"

# ── 1. Try to download a pre-built binary ──────────────────────────
$IsWinArm = [Environment]::Is64BitOperatingSystem -eq $false
$Arch = if ($IsWinArm) { "arm64" } else { "amd64" }
$BinaryName = "synthgraph-windows-$Arch.exe"

if ($Version -eq "latest") {
  try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name
    $Asset = $Release.assets | Where-Object { $_.name -eq $BinaryName }
    if ($Asset) {
      $OutDir = "$env:USERPROFILE\.synthgraph\bin"
      $OutFile = "$OutDir\synthgraph.exe"
      New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
      Write-Host "Downloading SynthGraph $Version for Windows-$Arch ..."
      Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $OutFile
      Write-Host "Installed to $OutFile"
      Write-Host ""
      Write-Host "Add to your PATH (run this once):"
      Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';$OutDir', 'User')"
      exit 0
    }
  } catch {
    # No release or no matching asset — fall through to build
  }
}

# ── 2. Build from source (requires Go + GCC) ───────────────────────
$HasGo = $null -ne (Get-Command "go" -ErrorAction SilentlyContinue)
if (-not $HasGo) {
  Write-Host "No pre-built binary for Windows-$Arch yet, and Go is not installed."
  Write-Host ""
  Write-Host "To build from source, install these first:"
  Write-Host "  1. Go:      https://go.dev/dl/"
  Write-Host "  2. GCC:     https://www.msys2.org/  (then: pacman -S mingw-w64-ucrt-x86_64-gcc)"
  Write-Host "  3. Add C:\msys64\ucrt64\bin to your PATH"
  Write-Host ""
  Write-Host "Then run:"
  Write-Host "  git clone https://github.com/$Repo.git"
  Write-Host "  cd SynthGraph"
  Write-Host "  .\dev.ps1 build"
  Write-Host "  .\dev.ps1 run web"
  exit 1
}

$HasGcc = $null -ne (Get-Command "gcc" -ErrorAction SilentlyContinue)
if (-not $HasGcc) {
  Write-Host "Go is installed, but GCC is required for CGO (PostgreSQL parser)."
  Write-Host "Install MSYS2: https://www.msys2.org/"
  Write-Host "Then: pacman -S mingw-w64-ucrt-x86_64-gcc"
  Write-Host "And add C:\msys64\ucrt64\bin to your PATH."
  exit 1
}

$TmpDir = "$env:TEMP\synthgraph-install"
if (Test-Path $TmpDir) { Remove-Item -Recurse -Force $TmpDir }
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

Write-Host "Building SynthGraph from source (this takes a minute)..."
git clone -q "https://github.com/$Repo.git" "$TmpDir\src" 2>$null
if (-not $?) {
  Write-Host "Failed to clone repository. Make sure Git is installed: https://git-scm.com/"
  exit 1
}

$env:CGO_ENABLED = "1"
Push-Location "$TmpDir\src"
try {
  go build -o "$TmpDir\synthgraph.exe" ./cmd/synthgraph/
  go build -o "$TmpDir\synthgraph-web.exe" ./cmd/synthgraph-web/
} finally {
  Pop-Location
}

$OutDir = "$env:USERPROFILE\.synthgraph\bin"
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
Copy-Item "$TmpDir\synthgraph.exe" "$OutDir\synthgraph.exe" -Force
Copy-Item "$TmpDir\synthgraph-web.exe" "$OutDir\synthgraph-web.exe" -Force
Remove-Item -Recurse -Force $TmpDir

Write-Host "Installed to $OutDir"
Write-Host ""
Write-Host "Add to your PATH (run this once):"
Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';$OutDir', 'User')"
Write-Host ""
Write-Host "Or just run directly from that folder:"
Write-Host "  & '$OutDir\synthgraph.exe' generate -i schema.sql"
