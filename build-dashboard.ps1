param([ValidateSet('windows', 'linux')][string]$Platform = 'windows')

# Build one self-contained x86-64 dashboard with the matching Linux SSH helper; ARM remains deferred.
$ErrorActionPreference = 'Stop'
$taskRoot = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
$taskOldDirectory = Get-Location
$taskOldGOOS = $env:GOOS
$taskOldGOARCH = $env:GOARCH
$taskOldCGO = $env:CGO_ENABLED
$taskOldCache = $env:GOCACHE
try {
    Set-Location -LiteralPath $taskRoot
    New-Item -ItemType Directory -Path (Join-Path $taskRoot 'build') -Force | Out-Null
    $env:GOCACHE = Join-Path $taskRoot '.cache\go-build'
    $env:GOARCH = 'amd64'
    $env:GOOS = 'linux'
    $env:CGO_ENABLED = '0'
    $taskHelper = Join-Path $taskRoot 'build\sneaky-port-server-linux-amd64'
    & go build -trimpath -o $taskHelper ./cmd/sneaky-port
    if ($LASTEXITCODE -ne 0) { throw 'Linux helper build failed' }
    $taskCompressed = [System.IO.File]::Create((Join-Path $taskRoot 'internal\control\helper-linux-amd64.gz'))
    try {
        $taskGzip = New-Object System.IO.Compression.GZipStream($taskCompressed, [System.IO.Compression.CompressionLevel]::Optimal)
        try {
            $taskInput = [System.IO.File]::OpenRead($taskHelper)
            try { $taskInput.CopyTo($taskGzip) } finally { $taskInput.Dispose() }
        } finally { $taskGzip.Dispose() }
    } finally { $taskCompressed.Dispose() }
    $env:GOOS = $Platform
    $taskName = if ($Platform -eq 'windows') { 'sneaky-port-dashboard.exe' } else { 'sneaky-port-dashboard-linux-amd64' }
    & go build -trimpath -tags dashboard_bundle -o (Join-Path $taskRoot ('build\' + $taskName)) ./cmd/sneaky-port
    if ($LASTEXITCODE -ne 0) { throw 'Packaged dashboard build failed' }
    Write-Output ('Built build/' + $taskName + ' with its matching Linux helper')
} finally {
    $env:GOOS = $taskOldGOOS
    $env:GOARCH = $taskOldGOARCH
    $env:CGO_ENABLED = $taskOldCGO
    $env:GOCACHE = $taskOldCache
    Set-Location -LiteralPath $taskOldDirectory
}
