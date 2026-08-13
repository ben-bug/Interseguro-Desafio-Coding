@echo off
setlocal EnableExtensions

for %%I in ("%~dp0..") do set "ROOT=%%~fI"

rem Las variables se definen una sola vez y los procesos hijos las heredan.
rem La forma set "NOMBRE=valor" evita espacios accidentales al final.
set "JWT_SECRET=secreto-local"
set "DEMO_USERNAME=demo"
set "DEMO_PASSWORD=demo1234"
set "STATS_API_URL=http://localhost:3000"

if /I "%~1"=="--check" goto check

call :validate_tools
if errorlevel 1 goto failed

call :ensure_dependencies
if errorlevel 1 goto failed

echo.
echo Levantando API Node ^(:3000^), API Go ^(:8080^) y Frontend ^(:5173^)...

where wt.exe >nul 2>&1
if errorlevel 1 goto separate_windows

echo Abriendo Windows Terminal con tres pestanas...
wt.exe -w new new-tab --title "IS API Node" --startingDirectory "%ROOT%\api-node" cmd.exe /k "npm run dev" ^; new-tab --title "IS API Go" --startingDirectory "%ROOT%\api-go" cmd.exe /k "go run ./cmd/server" ^; new-tab --title "IS Frontend" --startingDirectory "%ROOT%\frontend" cmd.exe /k "npm run dev"
if errorlevel 1 goto failed
goto started

:separate_windows
echo Windows Terminal no esta disponible. Abriendo tres ventanas independientes...
start "IS API Node" /D "%ROOT%\api-node" cmd.exe /k "npm run dev"
start "IS API Go" /D "%ROOT%\api-go" cmd.exe /k "go run ./cmd/server"
start "IS Frontend" /D "%ROOT%\frontend" cmd.exe /k "npm run dev"
goto started

:started
echo.
echo Servicios iniciados:
echo   Frontend:  http://localhost:5173
echo   API Go:    http://localhost:8080
echo   API Node:  http://localhost:3000
echo   Usuario:   demo
echo   Contrasena: demo1234
echo.
echo Cierra las pestanas o ventanas para detener los servicios.
pause
exit /b 0

:validate_tools
if not exist "%ROOT%\api-node\package.json" (
    echo ERROR: no se encontro api-node\package.json en "%ROOT%".
    exit /b 1
)
if not exist "%ROOT%\api-go\go.mod" (
    echo ERROR: no se encontro api-go\go.mod en "%ROOT%".
    exit /b 1
)
if not exist "%ROOT%\frontend\package.json" (
    echo ERROR: no se encontro frontend\package.json en "%ROOT%".
    exit /b 1
)
where npm.cmd >nul 2>&1
if errorlevel 1 (
    echo ERROR: npm no esta disponible en PATH. Instala Node.js 22 o superior.
    exit /b 1
)
where go.exe >nul 2>&1
if errorlevel 1 (
    if exist "%ProgramFiles%\Go\bin\go.exe" (
        set "PATH=%ProgramFiles%\Go\bin;%PATH%"
    ) else (
        echo ERROR: Go no esta disponible en PATH ni en "%ProgramFiles%\Go\bin".
        exit /b 1
    )
)
exit /b 0

:ensure_dependencies
if not exist "%ROOT%\api-node\node_modules" (
    echo Instalando dependencias de API Node...
    pushd "%ROOT%\api-node"
    call npm install
    if errorlevel 1 (
        popd
        exit /b 1
    )
    popd
)

if not exist "%ROOT%\frontend\node_modules" (
    echo Instalando dependencias del frontend...
    pushd "%ROOT%\frontend"
    call npm install
    if errorlevel 1 (
        popd
        exit /b 1
    )
    popd
)
exit /b 0

:check
echo Verificando launcher...
call :validate_tools
if errorlevel 1 goto failed
echo ROOT=[%ROOT%]
echo JWT_SECRET=[%JWT_SECRET%]
echo DEMO_USERNAME=[%DEMO_USERNAME%]
echo DEMO_PASSWORD=[%DEMO_PASSWORD%]
echo STATS_API_URL=[%STATS_API_URL%]
echo Launcher OK. No se inicio ningun proceso.
exit /b 0

:failed
echo.
echo No se pudieron iniciar los servicios. Revisa el error mostrado arriba.
exit /b 1
