#Requires -Version 5
# git-sync installer for Windows — downloads the latest release and runs 'git-sync install'.
$ErrorActionPreference = "Stop"

$Repo       = "basmulder03/git-projects-sync"
$InstallDir = "$env:LOCALAPPDATA\Programs\git-sync"

Write-Host "Fetching latest git-sync release..."
$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name -replace '^v', ''

$Asset = $Release.assets | Where-Object { $_.name -like "*windows_amd64.zip" } | Select-Object -First 1
if (-not $Asset) {
    Write-Error "No Windows amd64 asset found in release $($Release.tag_name)"
    exit 1
}

Write-Host "Downloading git-sync $Version..."
$Tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath()) -Name ([System.IO.Path]::GetRandomFileName())
try {
    $ZipPath = Join-Path $Tmp "git-sync.zip"
    Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $ZipPath -UseBasicParsing
    Expand-Archive -Path $ZipPath -DestinationPath $Tmp -Force

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $ExeSrc = Join-Path $Tmp "git-sync.exe"
    Copy-Item -Path $ExeSrc -Destination (Join-Path $InstallDir "git-sync.exe") -Force
} finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}

Write-Host "Installed to $InstallDir\git-sync.exe"
Write-Host ""
& "$InstallDir\git-sync.exe" install
