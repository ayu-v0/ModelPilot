param(
    [string]$Version = "dev",
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not available in PATH"
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$module = "github.com/ayu-v0/ModelPilot"

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe" },
    @{ GOOS = "linux";   GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; EXT = "" }
)

foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $name = "modelpilot-$($target.GOOS)-$($target.GOARCH)$($target.EXT)"
    $path = Join-Path $OutputDir $name
    Write-Host "Building $path"
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $path ./cmd/modelpilot
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

Write-Host "Build artifacts written to $OutputDir"
