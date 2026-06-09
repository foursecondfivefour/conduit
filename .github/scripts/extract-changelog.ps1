param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [string]$OutputPath = "release_notes.md"
)

$lines = Get-Content -Path "CHANGELOG.md" -Encoding UTF8
$escaped = [regex]::Escape($Version)
$header = "^## \[$escaped\]"
$inSection = $false
$out = New-Object System.Collections.Generic.List[string]

foreach ($line in $lines) {
    if ($line -match $header) {
        $inSection = $true
        $out.Add($line)
        continue
    }
    if ($inSection -and $line -match "^## \[") {
        break
    }
    if ($inSection) {
        $out.Add($line)
    }
}

if ($out.Count -eq 0) {
    throw "No CHANGELOG section found for version $Version"
}

$out | Set-Content -Path $OutputPath -Encoding UTF8
Write-Output "Wrote $($out.Count) lines to $OutputPath"
