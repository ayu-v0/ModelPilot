param(
    [string]$Repo = "https://github.com/ayu-v0/ModelPilot.git",
    [string]$Branch = "main",
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"

Write-Host "Formatting Go files..."
gofmt -w cmd internal pkg

Write-Host "Tidying modules..."
go mod tidy

Write-Host "Running tests..."
go test ./...

Write-Host "Building binaries..."
& "$PSScriptRoot\build.ps1" -Version $Version

Write-Host "Configuring git remote..."
$remotes = git remote
if ($remotes -contains "origin") {
    git remote set-url origin $Repo
} else {
    git remote add origin $Repo
}

git branch -M $Branch

git add .
$hasChanges = git status --porcelain
if ($hasChanges) {
    git commit -m "feat: integrate benchmark platform and release binaries"
} else {
    Write-Host "No source changes to commit."
}

Write-Host "Pushing to $Repo ..."
git push -u origin $Branch

Write-Host "Done. Binaries are in dist/. GitHub Actions will also build artifacts after push."
