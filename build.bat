@echo off
setlocal EnableDelayedExpansion

:: =============================================================================
:: Go Birthday - Build Script (Windows CMD)
:: =============================================================================
:: This script compiles the application for Windows using standard Batch commands.
:: It handles versioning and icon embedding.

set "BINARY_NAME=go-birthday.exe"
set "VERSION_FILE=VERSION"
set "MAIN_PKG_PATH=./cmd/go-birthday"
set "MAIN_PKG_DIR=cmd\go-birthday"

:: The Go package where we inject build variables.
set "CONFIG_PKG=github.com/tartampluch/go-birthday/internal/config"

:: -----------------------------------------------------------------------------
:: 1. Metadata Retrieval
:: -----------------------------------------------------------------------------
echo [1/3] Reading configuration...

:: Read the Version from file
if exist "%VERSION_FILE%" (
    set /p VERSION=<"%VERSION_FILE%"
    :: Trim whitespace from the version string
    for /f "tokens=* delims= " %%a in ("!VERSION!") do set VERSION=%%a
) else (
    set "VERSION=dev"
)

:: Retrieve Git Commit Hash (if Git is installed and we are in a repo)
set "COMMIT=none"
git rev-parse --short HEAD >nul 2>&1
if %errorlevel% equ 0 (
    for /f %%i in ('git rev-parse --short HEAD') do set COMMIT=%%i
)

echo      Target: %BINARY_NAME% v%VERSION% [%COMMIT%]

:: -----------------------------------------------------------------------------
:: 2. Resources (Icon)
:: -----------------------------------------------------------------------------
echo.
echo [2/3] Handling Windows resources...

:: Check if go-winres tool is available
go-winres >nul 2>&1
if %errorlevel% equ 0 (
    :: Generate the resource object files (.syso)
    go-winres make --product-version %VERSION% --file-version %VERSION%
    
    :: Move them to the main package folder so 'go build' includes them
    if exist "rsrc_windows_*.syso" (
        move /Y "rsrc_windows_*.syso" "%MAIN_PKG_DIR%\" >nul
    )
) else (
    :: Note: We must escape the closing parenthesis with ^ to avoid syntax errors in the block.
    echo      Warning: go-winres tool not found (Icon skipped^).
)

:: -----------------------------------------------------------------------------
:: 3. Compilation
:: -----------------------------------------------------------------------------
echo.
echo [3/3] Compiling...

:: Define Linker Flags:
:: -s -w : Optimize binary size (strip debug info).
:: -H=windowsgui : Hide console window.
:: -X : Inject Version and Commit variables.
set "LDFLAGS=-s -w -H=windowsgui -X '%CONFIG_PKG%.Version=%VERSION%' -X '%CONFIG_PKG%.Commit=%COMMIT%'"

:: Run code generation
go generate ./...
if %errorlevel% neq 0 exit /b %errorlevel%

:: Build the binary
go build -ldflags "%LDFLAGS%" -o "%BINARY_NAME%" "%MAIN_PKG_PATH%"
if %errorlevel% neq 0 (
    echo.
    echo ^>^> Build Failed!
    exit /b 1
)

echo.
echo ^>^> Build Success.
