#!/usr/bin/env pwsh
<#
.SYNOPSIS
  Dev helper for SynthGraph — build, test, lint, run, clean, CI.

.EXAMPLE
  .\dev.ps1 build              # build all binaries into bin/
  .\dev.ps1 test               # unit tests (no CGO)
  .\dev.ps1 test all           # full test suite (CGO required)
  .\dev.ps1 test coverage      # with coverage report
  .\dev.ps1 lint               # go vet + gofmt
  .\dev.ps1 run web            # start web server (port 8080)
  .\dev.ps1 run web 9090       # custom port
  .\dev.ps1 run cli            # CLI on ecommerce test schema
  .\dev.ps1 clean              # remove build artifacts
  .\dev.ps1 ci                 # full pipeline: vet > build > test
#>

param(
  [Parameter(Position = 0)]
  [string]$Command = 'build',

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ArgsList
)

$ROOT = Split-Path -Parent $PSCommandPath
$BIN  = Join-Path $ROOT 'bin'

function Build {
  Write-Host "=== Building synthgraph (CLI) ===" -ForegroundColor Cyan
  $env:CGO_ENABLED = '1'
  New-Item -ItemType Directory -Path $BIN -Force | Out-Null
  go build -o (Join-Path $BIN 'synthgraph.exe')   (Join-Path $ROOT 'cmd/synthgraph/')
  Write-Host "=== Building synthgraph-web ===" -ForegroundColor Cyan
  go build -o (Join-Path $BIN 'synthgraph-web.exe') (Join-Path $ROOT 'cmd/synthgraph-web/')
  Write-Host "`nDone — binaries in $BIN" -ForegroundColor Green
  Get-ChildItem $BIN | Select-Object Name, Length
}

function Test-Runner {
  $mode = if ($ArgsList.Count -gt 0) { $ArgsList[0] } else { 'unit' }
  $rest = if ($ArgsList.Count -gt 1) { $ArgsList[1..$ArgsList.Count] } else { @() }

  switch ($mode) {
    'unit' {
      Write-Host "=== Unit tests (no CGO) ===" -ForegroundColor Cyan
      go test ./internal/schema/... ./internal/graph/... ./internal/planner/... `
               ./internal/generator/... ./internal/exporter/... ./internal/semantic/... `
               -v -count=1 @rest
    }
    'all' {
      Write-Host "=== Full test suite (CGO) ===" -ForegroundColor Cyan
      $env:CGO_ENABLED = '1'
      go test ./... -v -count=1 @rest
    }
    'coverage' {
      Write-Host "=== Coverage report ===" -ForegroundColor Cyan
      $env:CGO_ENABLED = '1'
      go test ./... -coverprofile="$ROOT/coverage.out" -count=1 @rest
      go tool cover -func="$ROOT/coverage.out" | Select-Object -Last 1
      Write-Host "HTML: go tool cover -html=$ROOT/coverage.out" -ForegroundColor Gray
    }
    'quick' {
      Write-Host "=== Quick: non-CGO packages ===" -ForegroundColor Cyan
      go test ./internal/... ./cmd/synthgraph/... ./cmd/synthgraph-web/server/... -count=1 @rest
    }
    'server' {
      Write-Host "=== Server tests ===" -ForegroundColor Cyan
      go test ./cmd/synthgraph-web/server/... -v -count=1 @rest
    }
    default {
      Write-Host "=== Tests: $mode ===" -ForegroundColor Cyan
      $env:CGO_ENABLED = '1'
      go test $mode -v -count=1 @rest
    }
  }
}

function Lint {
  Write-Host "=== go vet ===" -ForegroundColor Cyan
  go vet ./...
  Write-Host "=== gofmt ===" -ForegroundColor Cyan
  $unformatted = & gofmt -l $ROOT | Where-Object { $_ -notmatch '^vendor/' }
  if ($unformatted) {
    Write-Host "Files need formatting:" -ForegroundColor Red
    $unformatted
    exit 1
  }
  Write-Host "All files formatted." -ForegroundColor Green
  Write-Host "`nLint passed!" -ForegroundColor Green
}

function Run {
  if (-not (Test-Path (Join-Path $BIN 'synthgraph.exe')) -or
      -not (Test-Path (Join-Path $BIN 'synthgraph-web.exe'))) {
    Build
  }

  $sub = if ($ArgsList.Count -gt 0) { $ArgsList[0] } else { 'web' }
  switch ($sub) {
    'web' {
      $port = if ($ArgsList.Count -gt 1) { $ArgsList[1] } else { '8080' }
      Write-Host "==> Starting synthgraph-web on port $port" -ForegroundColor Green
      & (Join-Path $BIN 'synthgraph-web.exe') --port $port
    }
    'cli' {
      $schema = if ($ArgsList.Count -gt 1) { $ArgsList[1] } else { Join-Path $ROOT 'testdata/schemas/ecommerce.sql' }
      Write-Host "==> Running CLI with schema: $schema" -ForegroundColor Green
      & (Join-Path $BIN 'synthgraph.exe') generate --input $schema --rows 50 --output /dev/stdout
    }
    default {
      Write-Host "Unknown run target: $sub" -ForegroundColor Red
      Write-Host "Usage: .\dev.ps1 run [web|cli]"
    }
  }
}

function Clean {
  Write-Host "=== Removing bin/ ===" -ForegroundColor Cyan
  Remove-Item -Recurse -Force $BIN -ErrorAction SilentlyContinue
  Write-Host "=== Removing coverage.out ===" -ForegroundColor Cyan
  Remove-Item -Force "$ROOT/coverage.out" -ErrorAction SilentlyContinue
  Write-Host "=== Go clean ===" -ForegroundColor Cyan
  go clean -cache ./...
  Write-Host "Clean!" -ForegroundColor Green
}

function CI {
  Write-Host "===================================" -ForegroundColor Magenta
  Write-Host "  SynthGraph CI Pipeline"            -ForegroundColor Magenta
  Write-Host "===================================" -ForegroundColor Magenta

  Write-Host "`n=== Step 1: go vet ===" -ForegroundColor Cyan
  go vet ./...

  Write-Host "`n=== Step 2: Build ===" -ForegroundColor Cyan
  Build

  Write-Host "`n=== Step 3: Full test suite ===" -ForegroundColor Cyan
  $env:CGO_ENABLED = '1'
  go test ./... -race -timeout=180s -count=1

  Write-Host "`n===================================" -ForegroundColor Magenta
  Write-Host "  All checks passed!"                 -ForegroundColor Magenta
  Write-Host "===================================" -ForegroundColor Magenta
}

switch ($Command) {
  'build'    { Build }
  'test'     { Test-Runner }
  'lint'     { Lint }
  'run'      { Run }
  'clean'    { Clean }
  'ci'       { CI }
  default    {
    Write-Host "Unknown command: $Command" -ForegroundColor Red
    Write-Host "Usage: .\dev.ps1 <build|test|test all|test coverage|lint|run web|run cli|clean|ci>"
  }
}
