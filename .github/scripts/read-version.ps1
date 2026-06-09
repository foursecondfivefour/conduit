# Reads Conduit version from internal/config/version.go (single source of truth).
$line = Select-String -Path "internal/config/version.go" -Pattern 'Version = "([^"]+)"' | Select-Object -First 1
if (-not $line) { throw "Could not read version from internal/config/version.go" }
$version = $line.Matches.Groups[1].Value
Write-Output $version
