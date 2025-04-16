param(
  [string]$Version = "dev",
  [string]$OutDir = "dist",
  [string[]]$Targets = @("windows/amd64"),
  [switch]$Embedded = $false,
  [string]$EmbeddedTags = "whisper_embedded,sherpa_embedded"
)

$ErrorActionPreference = "Stop"
$isWindowsHost = $env:OS -eq 'Windows_NT'
$supportedTargets = @{
  "windows/amd64" = @{ GOOS = "windows"; GOARCH = "amd64"; EXE = ".exe"; Builder = "wails" }
  "linux/amd64"   = @{ GOOS = "linux"; GOARCH = "amd64"; EXE = ""; Builder = "go" }
  "darwin/amd64"  = @{ GOOS = "darwin"; GOARCH = "amd64"; EXE = ""; Builder = "go" }
}

$resolvedTargets = New-Object System.Collections.Generic.List[object]
foreach ($target in $Targets) {
  $key = $target.Trim().ToLowerInvariant()
  if (-not $supportedTargets.ContainsKey($key)) {
    throw "unsupported target '$target'. supported targets: $($supportedTargets.Keys -join ', ')"
  }
  $resolvedTargets.Add($supportedTargets[$key])
}

if ($resolvedTargets.Count -eq 0) {
  throw "no build targets specified"
}

if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir | Out-Null }
$env:GOCACHE = Join-Path (Get-Location) '.gocache'
$env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
if (-not (Test-Path $env:GOCACHE)) { New-Item -ItemType Directory -Path $env:GOCACHE | Out-Null }
if (-not (Test-Path $env:GOMODCACHE)) { New-Item -ItemType Directory -Path $env:GOMODCACHE | Out-Null }
$commit = ""
try { $commit = (git rev-parse --short HEAD) } catch { $commit = "unknown" }
$ldflags = "-X main.version=$Version -X main.commit=$commit"
$consoleDist = Join-Path 'frontend' 'dist/console'
$packageNames = @()
foreach ($t in $resolvedTargets) {
  $packageDir = Join-Path $OutDir "Norma-$($t.GOOS)-$($t.GOARCH)"
  $packageNames += "Norma-$($t.GOOS)-$($t.GOARCH)"
  if (Test-Path $packageDir) { Remove-Item -Recurse -Force $packageDir }
  New-Item -ItemType Directory -Path $packageDir | Out-Null
  $binaryName = if ($t.GOOS -eq 'windows') { "Norma.exe" } else { "Norma" }
  $out = Join-Path $packageDir $binaryName
  Write-Host "Building $($t.GOOS)-$($t.GOARCH) ..."

	if ($t.GOOS -eq 'windows') {
		if ($Embedded) {
			$env:CGO_ENABLED = "1"
			$env:GOFLAGS = "-tags=$EmbeddedTags"
			Write-Host "Embedded Wails build tags: $EmbeddedTags"
		}
		& wails build -clean -m -nopackage -trimpath -platform windows/amd64 -ldflags $ldflags -o Norma
		if ($LASTEXITCODE -ne 0) { throw "wails build failed for windows-amd64" }
		$wailsBinary = if (Test-Path 'build/bin/Norma.exe') { 'build/bin/Norma.exe' } else { 'build/bin/Norma' }
		Copy-Item -Force $wailsBinary $out
		if (-not (Test-Path $consoleDist)) { throw "console dist missing: $consoleDist" }
	} else {
		if ($isWindowsHost) {
			throw "target $($t.GOOS)/$($t.GOARCH) requires non-Windows build environment or dedicated cross-compile toolchain; use scripts/build.sh on the target platform or pass -Targets windows/amd64"
		}
		if (-not (Test-Path $consoleDist)) {
			Push-Location frontend
			try {
				& npm run build
				if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
			} finally {
				Pop-Location
			}
		}
		$env:GOOS = $t.GOOS
		$env:GOARCH = $t.GOARCH
		& go build -trimpath -ldflags $ldflags -o $out .
    if ($LASTEXITCODE -ne 0) { throw "go build failed for $($t.GOOS)-$($t.GOARCH)" }
  }

	Copy-Item -Force config.example.yaml (Join-Path $packageDir 'config.example.yaml')
	Copy-Item -Force config.example.yaml (Join-Path $packageDir 'config.yaml')
	Copy-Item -Recurse -Force $consoleDist (Join-Path $packageDir 'console')

	$resourceRoot = Join-Path $packageDir 'resource'
  New-Item -ItemType Directory -Force -Path $resourceRoot | Out-Null
  foreach ($dir in @('data','logs','diagnostics','runtime/providers')) {
    New-Item -ItemType Directory -Force -Path (Join-Path $resourceRoot $dir) | Out-Null
  }
  if (Test-Path 'resource/model') {
    Copy-Item -Recurse -Force 'resource/model' (Join-Path $resourceRoot 'model')
  }
  if ($t.GOOS -eq 'windows') {
    $kokoroSource = Join-Path 'build' 'embedded\tts\kokoro\windows'
    if (-not (Test-Path $kokoroSource)) {
      throw "embedded kokoro assets missing: $kokoroSource"
    }
    $kokoroTarget = Join-Path $resourceRoot 'runtime\providers\tts\current'
    New-Item -ItemType Directory -Force -Path $kokoroTarget | Out-Null
    Copy-Item -Recurse -Force (Join-Path $kokoroSource '*') $kokoroTarget
  }
}
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:GOFLAGS -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "Build complete. Output => $OutDir"
powershell -ExecutionPolicy Bypass -File scripts/write-release-manifest.ps1 -OutDir $OutDir -Version $Version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Running release layout check ..."
powershell -ExecutionPolicy Bypass -File scripts/check-release-layout.ps1 -OutDir $OutDir -Packages $packageNames
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
