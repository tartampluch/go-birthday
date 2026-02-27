<#
.SYNOPSIS
    Production Build Script for Go Birthday (Windows/PowerShell).

.DESCRIPTION
    This script automates the complete release process for the application:
    1. Metadata: Reads the VERSION file and retrieves Git Commit/Date.
    2. Resources: Embeds the application icon and manifest using 'go-winres'.
    3. Compilation: Builds the Go binary with optimizations (-s -w) and variable injection.

.NOTES
    - Prerequisites: Go 1.25+, Git, and optionally 'go-winres'.
    - To install go-winres: go install github.com/tc-hib/go-winres@latest
#>

# ------------------------------------------------------------------------------
# 0. Configuration
# ------------------------------------------------------------------------------

# "Fail-Fast": Stop execution immediately if any command returns an error.
$ErrorActionPreference = "Stop"

$BinaryName      = "go-birthday.exe"
$VersionFile     = ".\VERSION"
$MainPackagePkg  = "./cmd/go-birthday"    # Go package path for the build command
$MainPackageDir  = "cmd\go-birthday"      # Directory path for file operations

# The Go package path where global constants are defined.
$ConfigPkg       = "github.com/tartampluch/go-birthday/internal/config"

# ------------------------------------------------------------------------------
# 1. Metadata Retrieval
# ------------------------------------------------------------------------------
Write-Host "[1/3] Preparing build metadata..." -ForegroundColor Cyan

# Read the Semantic Version from the file.
if (Test-Path $VersionFile) {
    $Version = (Get-Content $VersionFile -Raw).Trim()
} else {
    $Version = "dev"
    Write-Warning "File 'VERSION' not found. Defaulting to '$Version'."
}

# Retrieve the current Git Commit Hash.
try {
    $Commit = (git rev-parse --short HEAD).Trim()
} catch {
    $Commit = "none"
    Write-Warning "Git repository not detected. Commit set to '$Commit'."
}

# Get the current date in UTC (ISO 8601 format).
$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "      Version: $Version" -ForegroundColor Gray
Write-Host "      Commit:  $Commit" -ForegroundColor Gray
Write-Host "      Date:    $Date" -ForegroundColor Gray

# ------------------------------------------------------------------------------
# 2. Windows Resources (Icon embedding)
# ------------------------------------------------------------------------------
Write-Host "`n[2/3] Embedding Windows resources..." -ForegroundColor Cyan

# Check if the 'go-winres' tool is available in the PATH.
if (Get-Command "go-winres" -ErrorAction SilentlyContinue) {
    # Generate the .syso files (system objects) containing the icon.
    go-winres make --product-version $Version --file-version $Version
    
    # Move the generated .syso files to the main package directory so the compiler finds them.
    $SysoFiles = Get-ChildItem -Filter "rsrc_windows_*.syso"
    if ($SysoFiles) {
        Move-Item -Path $SysoFiles.FullName -Destination $MainPackageDir -Force
        Write-Host "      Resources generated and moved successfully." -ForegroundColor Green
    }
} else {
    Write-Warning "Tool 'go-winres' is missing. The executable will not have an icon."
    Write-Warning "Run: go install github.com/tc-hib/go-winres@latest"
}

# ------------------------------------------------------------------------------
# 3. Compilation
# ------------------------------------------------------------------------------
Write-Host "`n[3/3] Compiling binary..." -ForegroundColor Cyan

# Construct the Linker Flags (LDFLAGS):
# -s -w : Strip debug information to reduce file size.
# -H=windowsgui : Hide the console window on startup (GUI mode).
# -X : Inject the metadata variables into the internal/config package.
$LdFlags =  "-s -w -H=windowsgui " +
            "-X '$ConfigPkg.Version=$Version' " +
            "-X '$ConfigPkg.Commit=$Commit' " +
            "-X '$ConfigPkg.Date=$Date'"

# Run 'go generate' first (standard practice).
go generate ./...

# Build the final executable.
go build -ldflags $LdFlags -o $BinaryName $MainPackagePkg

# Final check.
if (Test-Path $BinaryName) {
    Write-Host "`n>> Build Success: $BinaryName" -ForegroundColor Green
} else {
    Write-Error "Build failed. The artifact was not created."
}
