param(
    [Parameter(Mandatory = $true)][ValidateSet('windows', 'linux')][string]$Platform,
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$Repository = 'rashidbestry/sneaky-port'
)

# Build and verify one public x86-64 release archive; this does not establish live-network acceptance.
$ErrorActionPreference = 'Stop'
if ($Version -cnotmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$') {
    throw 'Version must be a v-prefixed semantic version, for example v0.1.0-alpha.1.'
}
if ($Matches[4]) {
    foreach ($taskIdentifier in $Matches[4].Split('.')) {
        if ($taskIdentifier -match '^0[0-9]+$') { throw 'Numeric prerelease identifiers cannot have leading zeroes.' }
    }
}
if ($Repository -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$') {
    throw 'Repository must be a safe GitHub owner/name.'
}
$taskRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$taskOldDirectory = Get-Location
$taskOldEnvironment = @{}
foreach ($taskVariable in @('GOOS', 'GOARCH', 'GOAMD64', 'CGO_ENABLED', 'GOCACHE', 'GOFLAGS', 'DASHBOARD_BUNDLE_ARTIFACT')) {
    $taskOldEnvironment[$taskVariable] = [Environment]::GetEnvironmentVariable($taskVariable, 'Process')
}

# Write UTF-8 text without a BOM consistently on Windows PowerShell 5.1 and PowerShell 7.
function Write-ReleaseText {
    param([string]$Path, [string]$Text)
    [System.IO.File]::WriteAllText($Path, $Text, (New-Object System.Text.UTF8Encoding($false)))
}

# Inspect actual binary build settings rather than inferring them from requested environment variables.
function Get-ReleaseBinaryInfo {
    param([string]$Path, [string]$Name, [string]$ExpectedPlatform, [bool]$Dashboard)
    $taskBuildLines = @(& go version -m $Path)
    if ($LASTEXITCODE -ne 0) { throw "Cannot inspect binary $Name" }
    $taskSettings = [ordered]@{}
    foreach ($taskLine in $taskBuildLines) {
        if ($taskLine -match '^\s*build\s+([^=]+)=(.*)$') { $taskSettings[$Matches[1]] = $Matches[2] }
    }
    foreach ($taskSetting in @{ 'GOOS' = $ExpectedPlatform; 'GOARCH' = 'amd64'; 'GOAMD64' = 'v1'; 'CGO_ENABLED' = '0'; '-trimpath' = 'true'; 'vcs.revision' = $taskRevision }.GetEnumerator()) {
        if ($taskSettings[$taskSetting.Key] -ne $taskSetting.Value) { throw "Unexpected $($taskSetting.Key) in $Name" }
    }
    if ($Dashboard -and $taskSettings['-tags'] -ne 'dashboard_bundle') { throw 'Dashboard bundle tag is missing.' }
    return [ordered]@{
        path = $Name
        bytes = (Get-Item -LiteralPath $Path).Length
        sha256 = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
        build_settings = $taskSettings
    }
}

# Exercise public read-only commands only on a matching native host; receipt/network claims remain outside this check.
function Test-ReleaseCommands {
    param([string]$Directory)
    foreach ($taskBinary in $taskBinaryNames) {
        $taskExecutable = Join-Path $Directory $taskBinary
        & $taskExecutable adapters | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Adapters smoke failed: $taskBinary" }
        & $taskExecutable validate | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Built-in profile validation smoke failed: $taskBinary" }
    }
}

# Require an exact public file inventory and unchanged binary hashes after archive extraction.
function Test-ReleaseExtraction {
    param([string]$Directory)
    $taskPrefix = $Directory.TrimEnd([char[]]'\/') + [System.IO.Path]::DirectorySeparatorChar
    $taskActualFiles = @(Get-ChildItem -LiteralPath $Directory -File -Recurse | ForEach-Object {
        $_.FullName.Substring($taskPrefix.Length).Replace('\', '/')
    } | Sort-Object)
    $taskExpectedFiles = @($taskPublicFiles + $taskBinaryNames + @('README.md', 'BUILD_INFO.json', 'licenses/go-runtime-LICENSE') | Sort-Object)
    if (Compare-Object -ReferenceObject $taskExpectedFiles -DifferenceObject $taskActualFiles) { throw 'Extracted archive has an unexpected public file inventory.' }
    foreach ($taskBinary in $taskBinaryInfo) {
        $taskHash = (Get-FileHash -LiteralPath (Join-Path $Directory $taskBinary.path) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($taskHash -ne $taskBinary.sha256) { throw "Extracted binary hash differs: $($taskBinary.path)" }
    }
}

try {
    Set-Location -LiteralPath $taskRoot
    $taskHostOS = (& go env GOHOSTOS) -join ''
    if ($LASTEXITCODE -ne 0 -or $taskHostOS -notin @('windows', 'linux')) { throw 'Use a Windows or Linux Go host.' }
    $taskHostArch = (& go env GOHOSTARCH) -join ''
    if ($LASTEXITCODE -ne 0 -or $taskHostArch -ne 'amd64') { throw 'Release packaging requires an x86-64 host.' }
    $taskRevision = (& git rev-parse HEAD) -join ''
    if ($LASTEXITCODE -ne 0 -or $taskRevision -notmatch '^[0-9a-f]{40}$') { throw 'Cannot determine actual Git HEAD.' }
    $taskGitStatus = @(& git status --porcelain --untracked-files=normal)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Git dirty state.' }
    $taskDirty = $taskGitStatus.Count -gt 0
    $taskGoVersion = (& go version) -join ''
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go toolchain version.' }
    $taskGOROOT = (& go env GOROOT) -join ''
    if ($LASTEXITCODE -ne 0) { throw 'Cannot locate Go runtime license.' }
    $taskRuntimeLicense = Join-Path $taskGOROOT 'LICENSE'
    if (-not (Test-Path -LiteralPath $taskRuntimeLicense -PathType Leaf)) { throw 'Go runtime license is missing.' }
    $taskFolder = "sneaky-port-$Version-$Platform-amd64"
    $taskPlatformStage = Join-Path $taskRoot "build/release-stage/$Version/$Platform"
    $taskAssets = Join-Path $taskRoot 'build/release-assets'
    $taskExtension = if ($Platform -eq 'windows') { '.zip' } else { '.tar.gz' }
    $taskArchive = Join-Path $taskAssets ($taskFolder + $taskExtension)
    if ((Test-Path -LiteralPath $taskPlatformStage) -or (Test-Path -LiteralPath $taskArchive)) {
        throw 'Staging or archive already exists; inspect it or choose a new version instead of overwriting.'
    }
    $taskPublicFiles = @(
        'docs/THIRD_PARTY_NOTICES.md',
        'licenses/go-crypto-LICENSE', 'licenses/go-sys-LICENSE', 'licenses/go-term-LICENSE',
        'licenses/go-yaml-LICENSE', 'licenses/go-yaml-NOTICE', 'licenses/go-yaml-libyaml-LICENSE'
    )
    foreach ($taskPublicFile in $taskPublicFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $taskRoot $taskPublicFile) -PathType Leaf)) { throw "Missing public release file: $taskPublicFile" }
    }
    $env:CGO_ENABLED = '0'
    $env:GOARCH = 'amd64'
    $env:GOAMD64 = 'v1'
    $env:GOCACHE = Join-Path $taskRoot '.cache/go-build'
    $env:GOFLAGS = ''
    & (Join-Path $taskRoot 'build-dashboard.ps1') -Platform $Platform
    $taskHelper = Join-Path $taskRoot 'build/sneaky-port-server-linux-amd64'
    $env:DASHBOARD_BUNDLE_ARTIFACT = $taskHelper
    $env:GOOS = $taskHostOS
    & go test -count=1 -tags dashboard_bundle -run '^TestPackagedHelper$' ./internal/control
    if ($LASTEXITCODE -ne 0) { throw 'Packaged helper does not match the embedded dashboard helper.' }
    $taskBinaryNames = if ($Platform -eq 'windows') { @('sneaky-port-dashboard.exe') } else { @('sneaky-port-dashboard-linux-amd64', 'sneaky-port-server-linux-amd64') }
    $taskBinaryInfo = @()
    foreach ($taskBinary in $taskBinaryNames) {
        $taskBinaryInfo += Get-ReleaseBinaryInfo -Path (Join-Path $taskRoot "build/$taskBinary") -Name $taskBinary -ExpectedPlatform $Platform -Dashboard ($taskBinary -like '*dashboard*')
    }
    $taskHelperInfo = Get-ReleaseBinaryInfo -Path $taskHelper -Name 'sneaky-port-server-linux-amd64' -ExpectedPlatform 'linux' -Dashboard $false
    $taskStage = Join-Path $taskPlatformStage $taskFolder
    New-Item -ItemType Directory -Path $taskStage -Force | Out-Null
    foreach ($taskPublicFile in $taskPublicFiles) {
        $taskDestination = Join-Path $taskStage $taskPublicFile
        New-Item -ItemType Directory -Path (Split-Path -Parent $taskDestination) -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $taskRoot $taskPublicFile) -Destination $taskDestination
    }
    Copy-Item -LiteralPath $taskRuntimeLicense -Destination (Join-Path $taskStage 'licenses/go-runtime-LICENSE')
    $taskNotices = Get-Content -LiteralPath (Join-Path $taskStage 'docs/THIRD_PARTY_NOTICES.md') -Raw
    Write-ReleaseText -Path (Join-Path $taskStage 'docs/THIRD_PARTY_NOTICES.md') -Text ($taskNotices.TrimEnd() + "`n`nThe compiled Go runtime and standard library retain the [Go BSD-style license](../licenses/go-runtime-LICENSE), built with $taskGoVersion.`n")
    foreach ($taskBinary in $taskBinaryNames) {
        Copy-Item -LiteralPath (Join-Path $taskRoot "build/$taskBinary") -Destination $taskStage
        if ($Platform -eq 'linux' -and $taskHostOS -eq 'linux') {
            & chmod +x -- (Join-Path $taskStage $taskBinary)
            if ($LASTEXITCODE -ne 0) { throw "Cannot preserve executable mode for $taskBinary" }
        }
    }
    $taskStartCommand = if ($Platform -eq 'windows') {
        '.\sneaky-port-dashboard.exe'
    } else {
        "chmod +x ./sneaky-port-dashboard-linux-amd64 ./sneaky-port-server-linux-amd64`n./sneaky-port-dashboard-linux-amd64 client-api"
    }
    $taskReadme = Get-Content -LiteralPath (Join-Path $taskRoot 'docs/release-readme.md') -Raw
    $taskReadme = $taskReadme.Replace('@@VERSION@@', $Version).Replace('@@PLATFORM@@', $Platform).Replace('@@REPOSITORY@@', $Repository).Replace('@@REVISION@@', $taskRevision).Replace('@@START_COMMAND@@', $taskStartCommand)
    Write-ReleaseText -Path (Join-Path $taskStage 'README.md') -Text $taskReadme
    $taskNativeSmoke = $taskHostOS -eq $Platform
    if ($taskNativeSmoke) { Test-ReleaseCommands -Directory $taskStage }
    else { Write-Host "Native $Platform adapters/validate smoke skipped on $taskHostOS; cross-build is not native runtime evidence." }
    $taskInfo = [ordered]@{
        schema_version = 1
        release = $Version
        source_revision = $taskRevision
        source_dirty = $taskDirty
        source_url = "https://github.com/$Repository/tree/$taskRevision"
        toolchain = $taskGoVersion
        target = "$Platform/amd64"
        native_command_smoke = $(if ($taskNativeSmoke) { 'passed: adapters and validate' } else { "not run: $Platform target on $taskHostOS host" })
        embedded_helper_verified = $true
        bundled_linux_helper = $taskHelperInfo
        binaries = $taskBinaryInfo
        acceptance = 'Packaging and command smoke do not establish full platform or live-network acceptance.'
    }
    Write-ReleaseText -Path (Join-Path $taskStage 'BUILD_INFO.json') -Text ($taskInfo | ConvertTo-Json -Depth 10)
    New-Item -ItemType Directory -Path $taskAssets -Force | Out-Null
    if ($Platform -eq 'windows') {
        Compress-Archive -LiteralPath $taskStage -DestinationPath $taskArchive -CompressionLevel Optimal
    } else {
        & tar -czf $taskArchive -C $taskPlatformStage $taskFolder
        if ($LASTEXITCODE -ne 0) { throw 'Linux archive creation failed.' }
    }
    $taskVerification = Join-Path $taskPlatformStage 'verify'
    New-Item -ItemType Directory -Path $taskVerification | Out-Null
    if ($Platform -eq 'windows') {
        Expand-Archive -LiteralPath $taskArchive -DestinationPath $taskVerification
    } else {
        & tar -xzf $taskArchive -C $taskVerification
        if ($LASTEXITCODE -ne 0) { throw 'Linux archive extraction failed.' }
    }
    $taskExtracted = Join-Path $taskVerification $taskFolder
    Test-ReleaseExtraction -Directory $taskExtracted
    if ($taskNativeSmoke) { Test-ReleaseCommands -Directory $taskExtracted }
    Write-Host "Verified archive inventory, binary hashes and available native command smoke: $taskArchive"
    Write-Host ('Archive bytes: ' + (Get-Item -LiteralPath $taskArchive).Length)
    Write-Host ('Archive SHA-256: ' + (Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant())
} finally {
    foreach ($taskVariable in $taskOldEnvironment.Keys) { [Environment]::SetEnvironmentVariable($taskVariable, $taskOldEnvironment[$taskVariable], 'Process') }
    Set-Location -LiteralPath $taskOldDirectory
}
