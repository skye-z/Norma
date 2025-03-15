param(
    [ValidateSet('init','status','register-local','uninstall')]
    [string]$Action = 'status',
    [string]$Root = '.',
    [string]$ConfigPath = 'config.yaml',
    [ValidateSet('wake','asr','tts','embedding','llm')]
    [string]$Kind,
    [string]$Name,
    [string]$Version = 'current',
    [string]$BinaryPath,
    [string]$ModelPath,
    [string]$Endpoint
)

$ErrorActionPreference = 'Stop'

function Get-ConfigValue {
    param(
        [string[]]$Lines,
        [string]$Section,
        [string]$Key
    )

    $inSection = $false
    foreach ($line in $Lines) {
        if ($line -match '^\s*[#]') { continue }
        if ($line -match '^\S') {
            $name = $line.TrimEnd(':').Trim()
            $inSection = ($name -eq $Section)
            continue
        }
        if (-not $inSection) { continue }
        if ($line -match ('^\s+' + [Regex]::Escape($Key) + '\s*:\s*(.+)$')) {
            return $matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return ''
}

function Get-ProvidersRoot([string]$ConfigPath) {
    $cfg = Get-Content -Encoding utf8 $ConfigPath
    $providersRoot = Get-ConfigValue -Lines $cfg -Section 'providers' -Key 'root'
    if ([string]::IsNullOrWhiteSpace($providersRoot)) {
        $providersRoot = 'resource/runtime/providers'
    }
    return (Join-Path (Get-Location) $providersRoot)
}

function Ensure-Layout([string]$ProvidersRoot) {
    foreach ($dir in @('wake','asr','tts','embedding','llm')) {
        New-Item -ItemType Directory -Force -Path (Join-Path $ProvidersRoot $dir) | Out-Null
        New-Item -ItemType Directory -Force -Path (Join-Path $ProvidersRoot "$dir/current") | Out-Null
    }
}

function Get-ManifestPath([string]$ProvidersRoot) {
    return (Join-Path $ProvidersRoot 'manifest.json')
}

function Load-Manifest([string]$ProvidersRoot) {
    $path = Get-ManifestPath $ProvidersRoot
    if (-not (Test-Path $path)) {
        return [ordered]@{ version = 1; providers = @() }
    }
    $raw = Get-Content $path -Raw -Encoding utf8 | ConvertFrom-Json
    return [ordered]@{
        version = $raw.version
        providers = @($raw.providers)
    }
}

function Save-Manifest([string]$ProvidersRoot, $manifest) {
    $json = $manifest | ConvertTo-Json -Depth 6
    [IO.File]::WriteAllText((Get-ManifestPath $ProvidersRoot), $json, [Text.UTF8Encoding]::new($false))
}

Push-Location $Root
try {
    $providersRoot = Get-ProvidersRoot $ConfigPath
    Ensure-Layout $providersRoot

    switch ($Action) {
        'init' {
            $manifest = Load-Manifest $providersRoot
            Save-Manifest $providersRoot $manifest
            Write-Host "[OK] provider layout initialized: $providersRoot"
        }
        'status' {
            $manifest = Load-Manifest $providersRoot
            Write-Host "providers_root=$providersRoot"
            foreach ($dir in @('wake','asr','tts','embedding','llm')) {
                $current = Join-Path $providersRoot "$dir/current"
                $items = @()
                if (Test-Path $current) {
                    $items = Get-ChildItem $current -Force | Select-Object -ExpandProperty Name
                }
                Write-Host ("[{0}] current={1}" -f $dir, ($(if ($items.Count -gt 0) { $items -join ', ' } else { 'empty' })))
            }
            Write-Host "manifest_entries=$($manifest.providers.Count)"
        }
        'register-local' {
            if ([string]::IsNullOrWhiteSpace($Kind) -or [string]::IsNullOrWhiteSpace($Name)) {
                throw 'register-local requires -Kind and -Name'
            }
            $target = Join-Path $providersRoot "$Kind/current"
            if (-not [string]::IsNullOrWhiteSpace($BinaryPath)) {
                Copy-Item -Force $BinaryPath (Join-Path $target (Split-Path $BinaryPath -Leaf))
            }
            if (-not [string]::IsNullOrWhiteSpace($ModelPath)) {
                Copy-Item -Force $ModelPath (Join-Path $target (Split-Path $ModelPath -Leaf))
            }
            $manifest = Load-Manifest $providersRoot
            $manifest.providers = @($manifest.providers | Where-Object { $_.kind -ne $Kind -or $_.name -ne $Name })
            $manifest.providers += [pscustomobject]@{
                name = $Name
                version = $Version
                kind = $Kind
                binary = $(if ($BinaryPath) { Split-Path $BinaryPath -Leaf } else { '' })
                model = $(if ($ModelPath) { Split-Path $ModelPath -Leaf } else { '' })
                endpoint = $Endpoint
                installed = $true
            }
            Save-Manifest $providersRoot $manifest
            Write-Host "[OK] provider registered: $Kind/$Name"
        }
        'uninstall' {
            if ([string]::IsNullOrWhiteSpace($Kind)) {
                throw 'uninstall requires -Kind'
            }
            $target = Join-Path $providersRoot "$Kind/current"
            if (Test-Path $target) {
                Get-ChildItem $target -Force | Remove-Item -Recurse -Force
            }
            $manifest = Load-Manifest $providersRoot
            if (-not [string]::IsNullOrWhiteSpace($Name)) {
                $manifest.providers = @($manifest.providers | Where-Object { $_.kind -ne $Kind -or $_.name -ne $Name })
            } else {
                $manifest.providers = @($manifest.providers | Where-Object { $_.kind -ne $Kind })
            }
            Save-Manifest $providersRoot $manifest
            Write-Host "[OK] provider uninstalled: $Kind"
        }
    }
}
finally {
    Pop-Location
}
