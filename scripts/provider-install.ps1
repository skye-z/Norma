param(
    [Parameter(Mandatory = $true)]
    [string]$CatalogPath,
    [Parameter(Mandatory = $true)]
    [string]$ProviderId,
    [string]$ProvidersRoot = 'runtime/providers',
    [switch]$Uninstall = $false
)

$ErrorActionPreference = 'Stop'

function Ensure-Dir([string]$Path) {
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Load-Catalog([string]$Path) {
    return Get-Content $Path -Raw -Encoding utf8 | ConvertFrom-Json
}

function Load-Manifest([string]$Path) {
    if (-not (Test-Path $Path)) {
        return [ordered]@{ version = 1; providers = @() }
    }
    $raw = Get-Content $Path -Raw -Encoding utf8 | ConvertFrom-Json
    return [ordered]@{ version = $raw.version; providers = @($raw.providers) }
}

function Save-Manifest([string]$Path, $Manifest) {
    $json = $Manifest | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText($Path, $json, [Text.UTF8Encoding]::new($false))
}

function Download-File([string]$Url, [string]$Target) {
    Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing
}

Push-Location .
try {
    $ProvidersRoot = Join-Path (Get-Location) $ProvidersRoot
    Ensure-Dir $ProvidersRoot
    $catalog = Load-Catalog $CatalogPath
    $item = $catalog.items | Where-Object { $_.id -eq $ProviderId } | Select-Object -First 1
    if ($null -eq $item) {
        throw "provider not found in catalog: $ProviderId"
    }

    $target = Join-Path $ProvidersRoot "$($item.kind)/current"
    Ensure-Dir $target
    $manifestPath = Join-Path $ProvidersRoot 'manifest.json'
    $manifest = Load-Manifest $manifestPath

    if ($Uninstall) {
        $manifest.providers = @($manifest.providers | Where-Object { $_.id -ne $item.id })
        if (Test-Path $target) {
            if ($item.target_name) {
                $mainTarget = Join-Path $target $item.target_name
                if (Test-Path $mainTarget) { Remove-Item -Force $mainTarget }
            }
            if ($item.extra_files) {
                foreach ($extra in $item.extra_files) {
                    $extraTarget = Join-Path $target $extra.target_name
                    if (Test-Path $extraTarget) { Remove-Item -Force $extraTarget }
                }
            }
            if (-not $item.target_name -and -not $item.extra_files) {
                Get-ChildItem $target -Force | Remove-Item -Recurse -Force
            }
        }
        Save-Manifest $manifestPath $manifest
        Write-Host "[OK] provider removed: $($item.id)"
        exit 0
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("norma-provider-" + [guid]::NewGuid().ToString('N'))
    Ensure-Dir $tmp
    $archive = Join-Path $tmp ([IO.Path]::GetFileName(($item.source -split '\?')[0]))

    Write-Host "[STEP] download $($item.source)"
    Download-File -Url $item.source -Target $archive

    Write-Host "[STEP] install to $target"
    switch ($item.archive_type) {
        'zip' {
            Expand-Archive -Path $archive -DestinationPath $tmp -Force
            $sourceRoot = $tmp
            $children = Get-ChildItem $tmp -Directory
            if ($children.Count -eq 1) {
                $sourceRoot = $children[0].FullName
            }
            Get-ChildItem $sourceRoot -Force | ForEach-Object {
                Copy-Item -Path $_.FullName -Destination (Join-Path $target $_.Name) -Recurse -Force
            }
        }
        'tar.bz2' {
            tar -xf $archive -C $tmp
            $sourceRoot = $tmp
            $children = Get-ChildItem $tmp -Directory
            if ($children.Count -ge 1) {
                $sourceRoot = $children[0].FullName
            }
            Get-ChildItem $sourceRoot -Force | ForEach-Object {
                Copy-Item -Path $_.FullName -Destination (Join-Path $target $_.Name) -Recurse -Force
            }
        }
        'file' {
            $targetName = if ($item.target_name) { $item.target_name } else { [IO.Path]::GetFileName(($item.source -split '\?')[0]) }
            Copy-Item -Force $archive (Join-Path $target $targetName)
        }
        default {
            Copy-Item -Force $archive (Join-Path $target ([IO.Path]::GetFileName($archive)))
        }
    }

    if ($item.extra_files) {
        foreach ($extra in $item.extra_files) {
            $extraArchive = Join-Path $tmp ([IO.Path]::GetFileName(($extra.source -split '\?')[0]))
            Write-Host "[STEP] download extra $($extra.source)"
            Download-File -Url $extra.source -Target $extraArchive
            $extraTarget = if ($extra.target_name) { $extra.target_name } else { [IO.Path]::GetFileName(($extra.source -split '\?')[0]) }
            Copy-Item -Force $extraArchive (Join-Path $target $extraTarget)
        }
    }

    $manifest.providers = @($manifest.providers | Where-Object { $_.id -ne $item.id })
    $manifest.providers += [pscustomobject]@{
        id = $item.id
        name = $item.name
        version = $item.version
        kind = $item.kind
        binary = ''
        model = $(if ($item.target_name) { $item.target_name } else { '' })
        endpoint = ''
        installed = $true
    }
    Save-Manifest $manifestPath $manifest

    Remove-Item -Recurse -Force $tmp
    Write-Host "[OK] provider installed: $($item.id)"
}
finally {
    Pop-Location
}
