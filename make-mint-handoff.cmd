@echo off
setlocal

set "SOURCE=%~dp0"
if "%SOURCE:~-1%"=="\" set "SOURCE=%SOURCE:~0,-1%"
set "DEST=%SOURCE%\mint handoof"
set "LIST_ONLY="
if /I "%~1"=="/L" set "LIST_ONLY=/L"

if not defined LIST_ONLY (
  if not exist "%DEST%" mkdir "%DEST%"
  if errorlevel 1 (
    echo Could not create destination: "%DEST%"
    exit /b 1
  )
)

robocopy "%SOURCE%" "%DEST%" /E /COPY:DAT /DCOPY:DAT /R:1 /W:1 /XJ %LIST_ONLY% ^
  /XD "%DEST%" "%SOURCE%\.git" "%SOURCE%\.task" "%SOURCE%\.local-tools" "%SOURCE%\.publish-gophoto" "%SOURCE%\.vscode" "%SOURCE%\bin" "%SOURCE%\log" "%SOURCE%\frontend\node_modules" "%SOURCE%\frontend\dist" "%SOURCE%\build\android\gen" "%SOURCE%\build\ios\xcode\gen" "%SOURCE%\build\linux\appimage\build" ^
  /XF "*.exe" "*.syso" "*.sqlite*" "*.log" "*.config" "config.yaml" "overlay.json" "settings.local.json" "gotohp.config" "upload cancel.json" "auto album test.json" "New album correct name and added-file count.json" "recen album not dragging.json" "3 auto album.json" "codex-clipboard-*.png"

set "RC=%ERRORLEVEL%"
if %RC% GEQ 8 (
  echo Robocopy failed with code %RC%.
  exit /b %RC%
)

if defined LIST_ONLY (
  echo Preview complete. No files were copied.
) else (
  echo Handoff copy complete: "%DEST%"
)
echo Robocopy code %RC% (codes 0-7 are success).
exit /b 0
