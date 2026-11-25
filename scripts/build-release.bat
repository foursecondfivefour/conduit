@echo off
cd /d "%~dp0.."
echo Running go mod tidy...
go mod tidy
echo Running go vet...
go vet ./...
echo Building conduit.exe...
go build -trimpath -ldflags="-s -w -H windowsgui" -o build/conduit.exe .
if errorlevel 1 goto :error
echo Building updater...
go build -trimpath -ldflags="-s -w -H windowsgui" -o build/conduit-updater.exe .\cmd\conduit-updater
if errorlevel 1 goto :error
goto :eof

:error
echo Build failed.
exit /b 1
