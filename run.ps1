#!/usr/bin/env pwsh
<#
.SYNOPSIS
  Build + run synthgraph-web or synthgraph CLI in a single command.
  No paths, no .exe files — just `go run` under the hood.

.EXAMPLE
  .\run.ps1 web              # web server on port 9090
  .\run.ps1 web 8080         # web server on port 8080
  .\run.ps1 cli              # CLI with sakila test schema
  .\run.ps1 cli my/schema.sql # CLI with custom schema
#>

param(
  [Parameter(Position = 0)]
  [string]$Target = 'web',

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ArgsList
)

$RED  = [ConsoleColor]::Red
$GRN  = [ConsoleColor]::Green
$CYN  = [ConsoleColor]::Cyan
$RST  = [ConsoleColor]::Gray

switch ($Target) {
  'web' {
    $port = if ($ArgsList.Count -gt 0) { $ArgsList[0] } else { '9090' }
    Write-Host "==> Building & starting synthgraph-web on port $port" -ForegroundColor $GRN
    go run ./cmd/synthgraph-web/ --port $port
  }
  'cli' {
    $schema = if ($ArgsList.Count -gt 0) { $ArgsList[0] } else { 'testdata/schemas/sakila.sql' }
    Write-Host "==> Building & running synthgraph CLI with: $schema" -ForegroundColor $GRN
    go run ./cmd/synthgraph/ generate --input $schema --rows 50 --output /dev/stdout
  }
  default {
    Write-Host "Usage: .\run.ps1 web [port] | cli [schema]" -ForegroundColor $RED
    exit 1
  }
}
