param(
    [ValidateSet("all", "cle", "cle-debug")]
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
Push-Location $root
try {
    $platforms = @(
        @{ OS = "windows"; Arch = "amd64" },
        @{ OS = "windows"; Arch = "arm64" },
        @{ OS = "linux"; Arch = "amd64" },
        @{ OS = "linux"; Arch = "arm64" },
        @{ OS = "darwin"; Arch = "amd64" },
        @{ OS = "darwin"; Arch = "arm64" }
    )
    $programs = if ($Target -eq "all") { @("cle", "cle-debug") } else { @($Target) }
    foreach ($program in $programs) {
        foreach ($platform in $platforms) {
            $directory = Join-Path $root ("dist\" + $platform.OS + "-" + $platform.Arch)
            New-Item -ItemType Directory -Force -Path $directory | Out-Null
            $extension = if ($platform.OS -eq "windows") { ".exe" } else { "" }
            $output = Join-Path $directory ($program + $extension)
            Write-Host ("Building {0} for {1}/{2}" -f $program, $platform.OS, $platform.Arch)
            $env:GOOS = $platform.OS
            $env:GOARCH = $platform.Arch
            $env:CGO_ENABLED = "0"
            & go build -trimpath -o $output ("./cmd/" + $program)
            if ($LASTEXITCODE -ne 0) {
                throw ("Build failed for {0}/{1}" -f $platform.OS, $platform.Arch)
            }
        }
    }
}
finally {
    if ($null -eq $previousGOOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $previousGOOS }
    if ($null -eq $previousGOARCH) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $previousGOARCH }
    if ($null -eq $previousCGO) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $previousCGO }
    Pop-Location
}
