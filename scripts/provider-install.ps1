param(
    [Parameter(Mandatory = $true)]
    [string]$CatalogPath,
    [Parameter(Mandatory = $true)]
    [string]$ProviderId,
    [string]$ProvidersRoot = 'resource/runtime/providers',
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
    $json = $Manifest | ConvertTo-Json -Depth 10
    [IO.File]::WriteAllText($Path, $json, [Text.UTF8Encoding]::new($false))
}

function Download-File([string]$Url, [string]$Target) {
    Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing
}

function Get-Sha256([string]$Path) {
    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Test-Sha256([string]$Path, [string]$Expected) {
    if ([string]::IsNullOrWhiteSpace($Expected)) {
        return
    }
    $actual = Get-Sha256 -Path $Path
    $expectedNorm = $Expected.Trim().ToLowerInvariant()
    if ($actual -ne $expectedNorm) {
        throw "sha256 mismatch for ${Path}: expected=$expectedNorm actual=$actual"
    }
}

function Resolve-Checksum([string]$FilePath, [string]$ChecksumSource, [string]$MatchName) {
    if ([string]::IsNullOrWhiteSpace($ChecksumSource) -or [string]::IsNullOrWhiteSpace($MatchName)) {
        return $null
    }
    Download-File -Url $ChecksumSource -Target $FilePath
    $lines = Get-Content $FilePath -Encoding utf8
    foreach ($line in $lines) {
        if ($line -match '^([a-fA-F0-9]{64})\s+[* ]?(.+)$') {
            $hash = $Matches[1].ToLowerInvariant()
            $name = ($Matches[2] | ForEach-Object { $_.Trim() })
            if ($name -eq $MatchName) {
                return $hash
            }
        }
    }
    throw "checksum entry not found for $MatchName in $ChecksumSource"
}

function Write-ManagedFiles($Files, [string]$Target) {
    if ($null -eq $Files) {
        return
    }
    foreach ($file in @($Files)) {
        $targetName = [string]$file.target_name
        if ([string]::IsNullOrWhiteSpace($targetName)) {
            continue
        }
        $content = [string]$file.content
        [IO.File]::WriteAllText((Join-Path $Target $targetName), $content, [Text.UTF8Encoding]::new($false))
    }
}

function Install-ExtraFiles($ExtraFiles, [string]$Tmp, [string]$Target) {
    $hashes = @()
    foreach ($extra in $ExtraFiles) {
        $extraArchive = Join-Path $Tmp ([IO.Path]::GetFileName(($extra.source -split '\?')[0]))
        Write-Host "[STEP] download extra $($extra.source)"
        Download-File -Url $extra.source -Target $extraArchive
        $extraExpected = $extra.sha256
        if ([string]::IsNullOrWhiteSpace($extraExpected) -and $extra.checksum_source -and $extra.checksum_match) {
            $checksumCache = Join-Path $Tmp ("checksum-" + [IO.Path]::GetFileName(($extra.checksum_source -split '\?')[0]))
            $extraExpected = Resolve-Checksum -FilePath $checksumCache -ChecksumSource $extra.checksum_source -MatchName $extra.checksum_match
        }
        Test-Sha256 -Path $extraArchive -Expected $extraExpected
        $extraTarget = if ($extra.target_name) { $extra.target_name } else { [IO.Path]::GetFileName(($extra.source -split '\?')[0]) }
        Copy-Item -Force $extraArchive (Join-Path $Target $extraTarget)
        $hashes += (Get-Sha256 -Path $extraArchive)
    }
    return ($hashes -join ',')
}

function Flatten-ProviderPayload([string]$Kind, [string]$Target) {
    if ($Kind -ne 'wake') {
        return
    }
    $required = @('encoder', 'decoder', 'joiner', 'tokens.txt')
    foreach ($entry in Get-ChildItem -Path $Target -Directory -Force) {
        $files = Get-ChildItem -Path $entry.FullName -Recurse -File -Force
        $hasWakeAssets = $false
        foreach ($file in $files) {
            $name = $file.Name.ToLowerInvariant()
            if ($name -like 'encoder-epoch-*.onnx' -or $name -eq 'encoder.int8.onnx' -or $name -eq 'encoder.onnx' -or
                $name -like 'decoder-epoch-*.onnx' -or $name -eq 'decoder.int8.onnx' -or $name -eq 'decoder.onnx' -or
                $name -like 'joiner-epoch-*.onnx' -or $name -eq 'joiner.int8.onnx' -or $name -eq 'joiner.onnx' -or
                $name -eq 'tokens.txt') {
                $hasWakeAssets = $true
                break
            }
        }
        if (-not $hasWakeAssets) {
            continue
        }
        foreach ($file in $files) {
            $dest = Join-Path $Target $file.Name
            if (-not (Test-Path $dest)) {
                Copy-Item -Force $file.FullName $dest
            }
        }
        Remove-Item -Recurse -Force $entry.FullName
    }
    Get-ChildItem -Path $Target -File -Force | Where-Object { $_.Extension -in @('.zip', '.bz2', '.tar') } | Remove-Item -Force -ErrorAction SilentlyContinue
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
            if ($item.managed_files) {
                foreach ($managed in $item.managed_files) {
                    $managedTarget = Join-Path $target $managed.target_name
                    if (Test-Path $managedTarget) { Remove-Item -Force $managedTarget }
                }
            }
            if (-not $item.target_name -and -not $item.extra_files -and -not $item.managed_files) {
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
    $expected = $item.sha256
    if ([string]::IsNullOrWhiteSpace($expected) -and $item.checksum_source -and $item.checksum_match) {
        $checksumCache = Join-Path $tmp ("checksum-" + [IO.Path]::GetFileName(($item.checksum_source -split '\?')[0]))
        $expected = Resolve-Checksum -FilePath $checksumCache -ChecksumSource $item.checksum_source -MatchName $item.checksum_match
    }
    Test-Sha256 -Path $archive -Expected $expected
    $mainHash = Get-Sha256 -Path $archive

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
            & tar -xf $archive -C $tmp
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

    $extraHashes = ''
    if ($item.extra_files) {
        $extraHashes = Install-ExtraFiles -ExtraFiles $item.extra_files -Tmp $tmp -Target $target
    }
    if ($item.managed_files) {
        Write-ManagedFiles -Files $item.managed_files -Target $target
    }

    Flatten-ProviderPayload -Kind $item.kind -Target $target

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
        integrity = [string]$item.integrity
        integrity_note = [string]$item.integrity_note
        source = [string]$item.source
        local_sha256 = $mainHash
        extra_local_sha256 = $extraHashes
        installed_at = (Get-Date).ToString('s')
    }
    Save-Manifest $manifestPath $manifest

    Remove-Item -Recurse -Force $tmp
    Write-Host "[OK] provider installed: $($item.id)"
}
finally {
    Pop-Location
}
