@echo off
set PLATFORM=%1
if "%PLATFORM%"=="" set PLATFORM=pc

echo Starting Blue Banded Bee development environment (platform: %PLATFORM%)...

REM Check if Docker is running
docker ps >nul 2>&1
if errorlevel 1 (
    echo Error: Docker Desktop is not running. Please start Docker Desktop first.
    echo Download: https://docs.docker.com/desktop/
    pause
    exit /b 1
)

REM Check if Supabase CLI is installed
supabase --version >nul 2>&1
if errorlevel 1 (
    echo Error: Supabase CLI is not installed.
    echo Install with: npm install -g supabase
    echo Or download from: https://supabase.com/docs/guides/cli
    pause
    exit /b 1
)

REM Configure Air for platform
if /i "%PLATFORM%"=="mac" (
    echo Configuring Air for Mac/Linux...
    powershell -Command "(gc .air.toml) -replace '^cmd = \"go build -o \\./tmp/main\\.exe', '# cmd = \"go build -o ./tmp/main.exe' -replace '^bin = \"tmp/main\\.exe\"', '# bin = \"tmp/main.exe\"' -replace '^# cmd = \"go build -o \\./tmp/main', 'cmd = \"go build -o ./tmp/main' -replace '^# bin = \"tmp/main\"', 'bin = \"tmp/main\"' | sc .air.toml"
) else (
    echo Configuring Air for Windows...
    powershell -Command "(gc .air.toml) -replace '^# cmd = \"go build -o \\./tmp/main\"', '# cmd = \"go build -o ./tmp/main\"' -replace '^# bin = \"tmp/main\"', '# bin = \"tmp/main\"' -replace '^cmd = \"go build -o \\./tmp/main\\.exe', 'cmd = \"go build -o ./tmp/main.exe' -replace '^bin = \"tmp/main\\.exe\"', 'bin = \"tmp/main.exe\"' | sc .air.toml"
)

REM Start Supabase (will be no-op if already running)
echo Starting local Supabase...
supabase start

REM Start Air with hot reloading
echo Starting development server with hot reloading...
air