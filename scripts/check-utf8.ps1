param(
    [string]$Root = ".",
    [switch]$FailOnBom = $true
)

$ErrorActionPreference = "Stop"

$textExtensions = @(
    ".md", ".txt", ".go", ".mod", ".sum", ".yaml", ".yml", ".json", ".toml", ".xml",
    ".js", ".ts", ".tsx", ".jsx", ".css", ".scss", ".html", ".sql", ".sh", ".ps1", ".bat"
)

$excludeDirs = @(
    ".git", "node_modules", ".gomodcache", ".gocache", "dist", "build", "diagnostics"
)

$utf8Strict = [System.Text.UTF8Encoding]::new($false, $true)
$invalid = New-Object System.Collections.Generic.List[string]
$bomFiles = New-Object System.Collections.Generic.List[string]

$files = Get-ChildItem -Path $Root -Recurse -File | Where-Object {
    $ext = [System.IO.Path]::GetExtension($_.FullName).ToLowerInvariant()
    if ($textExtensions -notcontains $ext) { return $false }

    foreach ($d in $excludeDirs) {
        if ($_.FullName -match "[\\/]$([Regex]::Escape($d))([\\/]|$)") { return $false }
    }
    return $true
}

foreach ($file in $files) {
    $bytes = [System.IO.File]::ReadAllBytes($file.FullName)

    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        $bomFiles.Add($file.FullName)
    }

    try {
        [void]$utf8Strict.GetString($bytes)
    }
    catch {
        $invalid.Add($file.FullName)
    }
}

if ($invalid.Count -gt 0) {
    Write-Host "[FAIL] Non-UTF-8 files detected:" -ForegroundColor Red
    $invalid | ForEach-Object { Write-Host " - $_" }
}

if ($FailOnBom -and $bomFiles.Count -gt 0) {
    Write-Host "[FAIL] UTF-8 BOM files detected:" -ForegroundColor Red
    $bomFiles | ForEach-Object { Write-Host " - $_" }
}

if ($invalid.Count -gt 0 -or ($FailOnBom -and $bomFiles.Count -gt 0)) {
    exit 1
}

if ($bomFiles.Count -gt 0) {
    Write-Host "[WARN] UTF-8 BOM files detected:" -ForegroundColor Yellow
    $bomFiles | ForEach-Object { Write-Host " - $_" }
}

Write-Host "[OK] UTF-8 check passed. Files checked: $($files.Count)"
