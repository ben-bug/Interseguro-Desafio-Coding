@echo off
rem ===========================================================================
rem  Levanta los tres servicios en local, cada uno en su propia consola.
rem
rem  Alternativa a Docker para desarrollar: los servidores recargan al guardar,
rem  cosa que no ocurre dentro de un contenedor. Para levantar el sistema tal
rem  como se despliega, usar `docker compose up` desde la raiz.
rem
rem  Uso:
rem    start-all.bat            levanta los tres servicios
rem    start-all.bat --check    solo diagnostica el entorno, no inicia nada
rem ===========================================================================

setlocal EnableExtensions EnableDelayedExpansion

for %%I in ("%~dp0..") do set "ROOT=%%~fI"

rem --- Puertos -------------------------------------------------------------
set "PORT_NODE=3000"
set "PORT_GO=8080"
set "PORT_WEB=5173"

rem --- Configuracion de desarrollo -----------------------------------------
rem  Este secreto es exclusivo del entorno local: el script nunca se despliega.
rem  En la nube, JWT_SECRET y DEMO_PASSWORD se cargan como variables de la
rem  plataforma y el contenedor se niega a arrancar sin ellas; ver
rem  entrypoint.sh, que a proposito no define ningun valor por defecto.
set "JWT_SECRET=secreto-solo-para-desarrollo-local"
set "DEMO_USERNAME=demo"
set "DEMO_PASSWORD=demo1234"
set "STATS_API_URL=http://localhost:%PORT_NODE%"

if /I "%~1"=="--check" goto :check
if /I "%~1"=="-c" goto :check

call :require_projects || goto :failed
call :require_tools    || goto :failed
call :require_ports    || goto :failed
call :install_deps     || goto :failed

echo.
echo Iniciando servicios...
call :launch || goto :failed

echo.
echo   Frontend   http://localhost:%PORT_WEB%
echo   API Go     http://localhost:%PORT_GO%
echo   API Node   http://localhost:%PORT_NODE%
echo.
echo   Usuario    %DEMO_USERNAME%
echo   Clave      %DEMO_PASSWORD%
echo.
echo Cierra las tres consolas para detener los servicios.
exit /b 0


rem ===========================================================================
rem  Comprobaciones
rem ===========================================================================

:require_projects
if not exist "%ROOT%\api-node\package.json" (
    echo ERROR: falta api-node\package.json en "%ROOT%".
    echo        Ejecuta este script desde la carpeta dev-launcher del repositorio.
    exit /b 1
)
if not exist "%ROOT%\api-go\go.mod" (
    echo ERROR: falta api-go\go.mod en "%ROOT%".
    exit /b 1
)
if not exist "%ROOT%\frontend\package.json" (
    echo ERROR: falta frontend\package.json en "%ROOT%".
    exit /b 1
)
exit /b 0

:require_tools
where npm.cmd >nul 2>&1
if errorlevel 1 (
    echo ERROR: npm no esta en el PATH. Instala Node.js 22 o superior.
    exit /b 1
)

rem  Go suele quedar fuera del PATH de la sesion aunque este instalado, porque
rem  el instalador lo agrega al PATH del sistema y las consolas ya abiertas no
rem  lo recargan. Se busca en la ruta habitual antes de darlo por ausente.
where go.exe >nul 2>&1
if errorlevel 1 (
    if exist "%ProgramFiles%\Go\bin\go.exe" (
        set "PATH=%ProgramFiles%\Go\bin;!PATH!"
        echo Aviso: Go se tomo de "%ProgramFiles%\Go\bin" ^(no estaba en el PATH^).
    ) else (
        echo ERROR: Go no esta en el PATH ni en "%ProgramFiles%\Go\bin".
        echo        Instalalo con:  winget install GoLang.Go
        exit /b 1
    )
)
exit /b 0

rem  Sin esta comprobacion, un puerto ocupado se traduce en tres consolas
rem  abiertas de las que solo una falla, con el error enterrado en su propia
rem  ventana. Es mas util detenerse antes y decir cual es el proceso.
:require_ports
set "PORTS_BUSY="
call :check_port %PORT_NODE% "API Node"
call :check_port %PORT_GO%   "API Go"
call :check_port %PORT_WEB%  "Frontend"

if defined PORTS_BUSY (
    echo.
    echo Libera esos puertos y vuelve a intentarlo. Para ver que los ocupa:
    echo   netstat -ano ^| findstr LISTENING
    echo Y para detener un proceso por su identificador:
    echo   taskkill /PID ^<pid^> /F
    exit /b 1
)
exit /b 0

:check_port
set "PORT_TO_CHECK=%~1"
set "PORT_OWNER=%~2"
set "FOUND_PID="

rem  El patron incluye los dos puntos y el estado para no confundir el 3000 con
rem  un 13000 ni contar conexiones salientes hacia ese puerto.
for /f "tokens=5" %%P in ('netstat -ano -p TCP ^| findstr /R /C:":%PORT_TO_CHECK% .*LISTENING"') do (
    set "FOUND_PID=%%P"
)

if defined FOUND_PID (
    set "PORTS_BUSY=1"
    for /f "tokens=1 delims=," %%N in ('tasklist /FI "PID eq !FOUND_PID!" /FO CSV /NH 2^>nul') do (
        echo ERROR: el puerto %PORT_TO_CHECK% ^(%PORT_OWNER%^) ya esta ocupado por %%~N ^(PID !FOUND_PID!^).
    )
)
exit /b 0

:install_deps
call :install_one "%ROOT%\api-node" "API Node" || exit /b 1
call :install_one "%ROOT%\frontend" "Frontend" || exit /b 1
exit /b 0

:install_one
if exist "%~1\node_modules" exit /b 0
echo Instalando dependencias de %~2...
pushd "%~1"
call npm install
if errorlevel 1 (
    popd
    echo ERROR: fallo la instalacion de dependencias de %~2.
    exit /b 1
)
popd
exit /b 0


rem ===========================================================================
rem  Arranque
rem ===========================================================================

rem  Windows Terminal agrupa los tres servicios en pestanas de una sola ventana,
rem  que es mas comodo de seguir. Si no esta disponible se abren tres consolas.
:launch
where wt.exe >nul 2>&1
if errorlevel 1 goto :launch_plain

wt.exe -w new new-tab --title "QR · API Node" --startingDirectory "%ROOT%\api-node" cmd.exe /k "npm run dev" ^; new-tab --title "QR · API Go" --startingDirectory "%ROOT%\api-go" cmd.exe /k "go run ./cmd/server" ^; new-tab --title "QR · Frontend" --startingDirectory "%ROOT%\frontend" cmd.exe /k "npm run dev"
if errorlevel 1 goto :launch_plain
exit /b 0

:launch_plain
echo Windows Terminal no esta disponible; se abriran tres consolas.
start "QR - API Node" /D "%ROOT%\api-node" cmd.exe /k "npm run dev"
start "QR - API Go"   /D "%ROOT%\api-go"   cmd.exe /k "go run ./cmd/server"
start "QR - Frontend" /D "%ROOT%\frontend" cmd.exe /k "npm run dev"
exit /b 0


rem ===========================================================================
rem  Diagnostico
rem ===========================================================================

:check
echo Comprobando el entorno...
echo.
call :require_projects || goto :failed
echo   [ok] Los tres proyectos estan en su sitio.
call :require_tools    || goto :failed
echo   [ok] npm y Go disponibles.
call :require_ports    || goto :failed
echo   [ok] Puertos %PORT_NODE%, %PORT_GO% y %PORT_WEB% libres.
echo.
echo   ROOT            %ROOT%
echo   STATS_API_URL   %STATS_API_URL%
echo   DEMO_USERNAME   %DEMO_USERNAME%
echo.
echo Entorno correcto. No se inicio ningun proceso.
exit /b 0

:failed
echo.
echo No se pudieron iniciar los servicios. Revisa el error de arriba.
exit /b 1
