@echo off
REM City2TABULA Windows Setup Script
REM This script provides the same functionality as the makefile for Windows users

if "%1"=="" goto help
if "%1"=="help" goto help
if "%1"=="configure" goto configure
if "%1"=="configure-manual" goto configure-manual
if "%1"=="build" goto build
if "%1"=="up" goto up
if "%1"=="down" goto down
if "%1"=="dev" goto dev
if "%1"=="create-db" goto create-db
if "%1"=="extract-features" goto extract-features
if "%1"=="reset-db" goto reset-db
if "%1"=="setup" goto setup
if "%1"=="quick-start" goto quick-start
if "%1"=="status" goto status
if "%1"=="logs" goto logs
if "%1"=="clean" goto clean
if "%1"=="clean-all" goto clean-all

echo Unknown command: %1
goto help

:help
echo City2TABULA Windows Commands
echo ============================
echo.
echo Usage: setup.bat ^<command^>
echo.
echo Docker Environment:
echo   build            Build the Docker environment
echo   up               Start the Docker environment
echo   down             Stop the Docker environment
echo   logs             View Docker logs
echo   status           Check container status
echo.
echo Application Commands:
echo   dev              Start development environment with shell
echo   create-db        Create database and setup schemas
echo   extract-features Extract building features
echo   reset-db         Reset the entire database
echo.
echo Complete Workflows:
echo   configure        Interactive setup: select country and enter password
echo   configure-manual Copy docker.env to .env for manual editing
echo   setup            Build environment, configure, and start containers
echo   quick-start      Complete setup and processing
echo.
echo Cleanup:
echo   clean            Stop containers and remove volumes
echo   clean-all        Remove containers, volumes, and images
echo.
echo Examples:
echo   setup.bat setup
echo   setup.bat dev
echo   setup.bat create-db
goto end

:configure
echo Interactive City2TABULA Configuration
echo ====================================
echo.
echo Copying base environment configuration...
copy environment\docker.env .env
echo Base configuration copied!
echo.

echo Available Countries:
echo ===================
echo  1) austria       - SRID: 31256 (MGI / Austria GK East)
echo  2) belgium       - SRID: 31370 (Belgian Lambert 72)
echo  3) cyprus        - SRID: 3879  (GRS 1980 / Cyprus TM)
echo  4) czechia       - SRID: 5514  (S-JTSK / Krovak East North)
echo  5) denmark       - SRID: 25832 (ETRS89 / UTM zone 32N)
echo  6) france        - SRID: 2154  (RGF93 / Lambert-93)
echo  7) germany       - SRID: 25832 (ETRS89 / UTM zone 32N)
echo  8) greece        - SRID: 2100  (GGRS87 / Greek Grid)
echo  9) hungary       - SRID: 23700 (EOV)
echo 10) ireland       - SRID: 29902 (Irish National Grid)
echo 11) italy         - SRID: 3003  (Monte Mario / Italy zone 1)
echo 12) netherlands   - SRID: 28992 (Amersfoort / RD New)
echo 13) norway        - SRID: 25833 (ETRS89 / UTM zone 33N)
echo 14) poland        - SRID: 2180  (ETRS89 / Poland CS2000 zone 5)
echo 15) serbia        - SRID: 3114  (Serbian 1970 / Serbian Grid)
echo 16) slovenia      - SRID: 3794  (Slovenia 1996 / Slovene National Grid)
echo 17) spain         - SRID: 25830 (ETRS89 / UTM zone 30N)
echo 18) sweden        - SRID: 3006  (SWEREF99 TM)
echo 19) united_kingdom - SRID: 27700 (OSGB 1936 / British National Grid)
echo.

:country_loop
set /p country_choice="Select country (1-19): "

if "%country_choice%"=="1" (
    set COUNTRY=austria
    set SRID=31256
    set SRS_NAME=MGI / Austria GK East
    goto country_selected
)
if "%country_choice%"=="2" (
    set COUNTRY=belgium
    set SRID=31370
    set SRS_NAME=Belgian Lambert 72
    goto country_selected
)
if "%country_choice%"=="3" (
    set COUNTRY=cyprus
    set SRID=3879
    set SRS_NAME=GRS 1980 / Cyprus TM
    goto country_selected
)
if "%country_choice%"=="4" (
    set COUNTRY=czechia
    set SRID=5514
    set SRS_NAME=S-JTSK / Krovak East North
    goto country_selected
)
if "%country_choice%"=="5" (
    set COUNTRY=denmark
    set SRID=25832
    set SRS_NAME=ETRS89 / UTM zone 32N
    goto country_selected
)
if "%country_choice%"=="6" (
    set COUNTRY=france
    set SRID=2154
    set SRS_NAME=RGF93 / Lambert-93
    goto country_selected
)
if "%country_choice%"=="7" (
    set COUNTRY=germany
    set SRID=25832
    set SRS_NAME=ETRS89 / UTM zone 32N
    goto country_selected
)
if "%country_choice%"=="8" (
    set COUNTRY=greece
    set SRID=2100
    set SRS_NAME=GGRS87 / Greek Grid
    goto country_selected
)
if "%country_choice%"=="9" (
    set COUNTRY=hungary
    set SRID=23700
    set SRS_NAME=EOV
    goto country_selected
)
if "%country_choice%"=="10" (
    set COUNTRY=ireland
    set SRID=29902
    set SRS_NAME=Irish National Grid
    goto country_selected
)
if "%country_choice%"=="11" (
    set COUNTRY=italy
    set SRID=3003
    set SRS_NAME=Monte Mario / Italy zone 1
    goto country_selected
)
if "%country_choice%"=="12" (
    set COUNTRY=netherlands
    set SRID=28992
    set SRS_NAME=Amersfoort / RD New
    goto country_selected
)
if "%country_choice%"=="13" (
    set COUNTRY=norway
    set SRID=25833
    set SRS_NAME=ETRS89 / UTM zone 33N
    goto country_selected
)
if "%country_choice%"=="14" (
    set COUNTRY=poland
    set SRID=2180
    set SRS_NAME=ETRS89 / Poland CS2000 zone 5
    goto country_selected
)
if "%country_choice%"=="15" (
    set COUNTRY=serbia
    set SRID=3114
    set SRS_NAME=Serbian 1970 / Serbian Grid
    goto country_selected
)
if "%country_choice%"=="16" (
    set COUNTRY=slovenia
    set SRID=3794
    set SRS_NAME=Slovenia 1996 / Slovene National Grid
    goto country_selected
)
if "%country_choice%"=="17" (
    set COUNTRY=spain
    set SRID=25830
    set SRS_NAME=ETRS89 / UTM zone 30N
    goto country_selected
)
if "%country_choice%"=="18" (
    set COUNTRY=sweden
    set SRID=3006
    set SRS_NAME=SWEREF99 TM
    goto country_selected
)
if "%country_choice%"=="19" (
    set COUNTRY=united_kingdom
    set SRID=27700
    set SRS_NAME=OSGB 1936 / British National Grid
    goto country_selected
)

echo Invalid selection. Please enter a number (1-19).
goto country_loop

:country_selected
echo.
echo Selected: %COUNTRY% (SRID: %SRID%)
echo.

echo Database Configuration:
echo =======================

REM Get username with default
set /p pg_user="Enter PostgreSQL username [default: postgres]: "
if "%pg_user%"=="" set pg_user=postgres

REM Get password using PowerShell for hidden input
echo Enter PostgreSQL password:
powershell -Command "$password = Read-Host -AsSecureString; $BSTR = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($password); $PlainPassword = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($BSTR); [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($BSTR); $PlainPassword" > temp_password.txt
set /p pg_password=<temp_password.txt
del temp_password.txt

echo.
echo Updating configuration file...

REM Update .env file using PowerShell for reliable text replacement
powershell -Command "(Get-Content .env) -replace 'COUNTRY=germany', 'COUNTRY=%COUNTRY%' | Set-Content .env"
powershell -Command "(Get-Content .env) -replace 'CITYDB_SRID=25832', 'CITYDB_SRID=%SRID%' | Set-Content .env"
powershell -Command "(Get-Content .env) -replace 'CITYDB_SRS_NAME=ETRS89 / UTM zone 32N', 'CITYDB_SRS_NAME=%SRS_NAME%' | Set-Content .env"
powershell -Command "(Get-Content .env) -replace 'DB_USER=postgres', 'DB_USER=%pg_user%' | Set-Content .env"
powershell -Command "(Get-Content .env) -replace '<your_pg_password>', '%pg_password%' | Set-Content .env"

echo Configuration completed!
echo.
echo Summary:
echo ========
echo Country: %COUNTRY%
echo SRID: %SRID%
echo SRS Name: %SRS_NAME%
echo Database: Configured
echo.
echo Next steps:
echo - Place your data in data\lod2\%COUNTRY%\ and data\lod3\%COUNTRY%\
echo - Run 'setup.bat up' to start containers
echo - Run 'setup.bat dev' to access development shell
goto end

:configure-manual
echo Manual configuration mode...
echo Copying environment configuration...
copy environment\docker.env .env
echo .env file created!
echo Please edit .env manually:
echo    - Set COUNTRY to your target country
echo    - Set CITYDB_SRID to the appropriate SRID
echo    - Set CITYDB_SRS_NAME to the appropriate SRS name
echo    - Replace '^<your_pg_password^>' with your PostgreSQL password
echo.
echo You can edit the file with:
echo   notepad .env
goto end

:build
echo Building Docker environment...
cd environment
docker compose build --no-cache
cd ..
goto end

:up
echo Starting Docker environment...
cd environment
docker compose up -d
cd ..
goto end

:down
echo Stopping Docker environment...
cd environment
docker compose down
cd ..
goto end

:dev
echo Starting development environment...
cd environment
docker compose up -d
docker exec -it city2tabula-environment bash
cd ..
goto end

:create-db
call :up
echo Creating database and setting up schemas...
cd environment
docker exec -it city2tabula-environment ./city2tabula -create_db
cd ..
goto end

:extract-features
call :up
echo Extracting building features...
cd environment
docker exec -it city2tabula-environment ./city2tabula -extract_features
cd ..
goto end

:reset-db
call :up
echo Resetting the entire database...
cd environment
docker exec -it city2tabula-environment ./city2tabula -reset_all
cd ..
goto end

:setup
echo Setting up City2TABULA environment...
call :build
call :configure
call :up
echo Environment is ready! Run 'setup.bat dev' to access the shell
goto end

:quick-start
echo Running complete City2TABULA pipeline...
call :setup
call :create-db
call :extract-features
echo Quick start complete!
goto end

:status
echo Checking container status...
cd environment
docker compose ps
cd ..
goto end

:logs
echo Viewing Docker logs...
cd environment
docker compose logs -f
cd ..
goto end

:clean
echo Stopping containers and removing volumes...
cd environment
docker compose down -v
cd ..
goto end

:clean-all
echo Removing containers, volumes, and images...
cd environment
docker compose down -v --rmi all
cd ..
goto end

:end