param([string]$Go = 'go')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$savedEnv = @{ CGO_ENABLED = $env:CGO_ENABLED; GOOS = $env:GOOS; GOARCH = $env:GOARCH }
$resourceTool = 'github.com/tc-hib/go-winres@v0.3.3'
Push-Location $projectRoot
try {
    $hostOS = & $Go env GOHOSTOS
    $hostArch = & $Go env GOHOSTARCH
    if ($LASTEXITCODE -ne 0) { throw 'Cannot detect Go host architecture' }
    $env:CGO_ENABLED = '0'
    $env:GOOS = $hostOS
    $env:GOARCH = $hostArch

    & $Go run $resourceTool make --in assets/winres.json --arch amd64,arm64 --out cmd/pulsebar/rsrc
    if ($LASTEXITCODE -ne 0) { throw 'Icon resource generation failed' }

    $stage = Join-Path $projectRoot '.artifacts/icon-build'
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    $temporaryExe = Join-Path $stage 'Samuel.exe'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    & $Go build -trimpath -ldflags '-s -w -H=windowsgui' -o $temporaryExe ./cmd/pulsebar
    if ($LASTEXITCODE -ne 0) { throw 'Temporary icon host build failed' }
    $env:GOOS = $hostOS
    $env:GOARCH = $hostArch
    & $Go run $resourceTool extract --dir $stage $temporaryExe
    if ($LASTEXITCODE -ne 0) { throw 'ICO export failed' }
    $icons = @(Get-ChildItem -LiteralPath $stage -Filter '*.ico' -File -Recurse)
    if ($icons.Count -ne 1) { throw 'Expected exactly one application icon' }
    Copy-Item -LiteralPath $icons[0].FullName -Destination assets/Samuel.ico -Force
    Get-Item -LiteralPath assets/Samuel.ico, cmd/pulsebar/rsrc_windows_amd64.syso, cmd/pulsebar/rsrc_windows_arm64.syso |
        Select-Object FullName, Length
} finally {
    foreach ($entry in $savedEnv.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, 'Process')
    }
    Pop-Location
}
