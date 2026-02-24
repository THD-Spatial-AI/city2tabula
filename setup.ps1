# City2TABULA PowerShell Setup Script
# This script provides the same functionality as the makefile for Windows PowerShell users

param(
    [Parameter(Position=0)]
    [string]$Command = "help"
)

function Show-Help {
    Write-Host "City2TABULA PowerShell Commands" -ForegroundColor Cyan
    Write-Host "================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\setup.ps1 <command>" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Docker Environment:" -ForegroundColor Green
    Write-Host "  build            Build the Docker environment"
    Write-Host "  up               Start the Docker environment"
    Write-Host "  down             Stop the Docker environment"
    Write-Host "  logs             View Docker logs"
    Write-Host "  status           Check container status"
    Write-Host ""
    Write-Host "Application Commands:" -ForegroundColor Green
    Write-Host "  dev              Start development environment with shell"
    Write-Host "  create-db        Create database and setup schemas"
    Write-Host "  extract-features Extract building features"
    Write-Host "  reset-db         Reset the entire database"
    Write-Host "  version (v)      Check City2TABULA version" -ForegroundColor Green
    Write-Host ""
    Write-Host "Complete Workflows:" -ForegroundColor Green
    Write-Host "  configure        Interactive setup: select country and enter password"
    Write-Host "  configure-manual Copy docker.env to .env for manual editing"
    Write-Host "  setup            Build environment, configure, and start containers"
    Write-Host "  quick-start      Complete setup and processing"
    Write-Host ""
    Write-Host "Cleanup:" -ForegroundColor Green
    Write-Host "  clean            Stop containers and remove volumes"
    Write-Host "  clean-all        Remove containers, volumes, and images"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Yellow
    Write-Host "  .\setup.ps1 setup"
    Write-Host "  .\setup.ps1 dev"
    Write-Host "  .\setup.ps1 create-db"
    Write-Host "  .\setup.ps1 version"
    Write-Host "  .\setup.ps1 v"
}

function Invoke-Configure {
    Write-Host "Interactive City2TABULA Configuration" -ForegroundColor Magenta
    Write-Host "=====================================" -ForegroundColor Magenta
    Write-Host ""

    # Country selection with mapping
    $countries = @{
        1 = @{Name="austria"; SRID="31256"; SRS="MGI / Austria GK East"}
        2 = @{Name="belgium"; SRID="31370"; SRS="Belgian Lambert 72"}
        3 = @{Name="cyprus"; SRID="3879"; SRS="GRS 1980 / Cyprus TM"}
        4 = @{Name="czechia"; SRID="5514"; SRS="S-JTSK / Krovak East North"}
        5 = @{Name="denmark"; SRID="25832"; SRS="ETRS89 / UTM zone 32N"}
        6 = @{Name="france"; SRID="2154"; SRS="RGF93 / Lambert-93"}
        7 = @{Name="germany"; SRID="25832"; SRS="ETRS89 / UTM zone 32N"}
        8 = @{Name="greece"; SRID="2100"; SRS="GGRS87 / Greek Grid"}
        9 = @{Name="hungary"; SRID="23700"; SRS="EOV"}
        10 = @{Name="ireland"; SRID="29902"; SRS="Irish National Grid"}
        11 = @{Name="italy"; SRID="3003"; SRS="Monte Mario / Italy zone 1"}
        12 = @{Name="netherlands"; SRID="28992"; SRS="Amersfoort / RD New"}
        13 = @{Name="norway"; SRID="25833"; SRS="ETRS89 / UTM zone 33N"}
        14 = @{Name="poland"; SRID="2180"; SRS="ETRS89 / Poland CS2000 zone 5"}
        15 = @{Name="serbia"; SRID="3114"; SRS="Serbian 1970 / Serbian Grid"}
        16 = @{Name="slovenia"; SRID="3794"; SRS="Slovenia 1996 / Slovene National Grid"}
        17 = @{Name="spain"; SRID="25830"; SRS="ETRS89 / UTM zone 30N"}
        18 = @{Name="sweden"; SRID="3006"; SRS="SWEREF99 TM"}
        19 = @{Name="united_kingdom"; SRID="27700"; SRS="OSGB 1936 / British National Grid"}
    }

    Write-Host "Available Countries:" -ForegroundColor Green
    Write-Host "===================" -ForegroundColor Green
    foreach ($key in $countries.Keys | Sort-Object) {
        $country = $countries[$key]
        Write-Host ("{0,2}) {1,-15} - SRID: {2,-5} ({3})" -f $key, $country.Name, $country.SRID, $country.SRS)
    }
    Write-Host ""

    do {
        $countryChoice = Read-Host "Select country (1-19)"
        $countryChoice = [int]$countryChoice -as [int]

        if ($countries.ContainsKey($countryChoice)) {
            $selectedCountry = $countries[$countryChoice]
            break
        } else {
            Write-Host "Invalid selection. Please enter a number (1-19)." -ForegroundColor Red
        }
    } while ($true)

    Write-Host ""
    Write-Host "Selected: $($selectedCountry.Name) (SRID: $($selectedCountry.SRID))" -ForegroundColor Green
    Write-Host ""

    # Get database credentials
    Write-Host "Database Configuration:" -ForegroundColor Green
    Write-Host "======================" -ForegroundColor Green

    # Get username with default
    $pgUser = Read-Host "Enter PostgreSQL username [default: postgres]"
    if ([string]::IsNullOrWhiteSpace($pgUser)) {
        $pgUser = "postgres"
    }

    # Get password
    do {
        $pgPassword = Read-Host "Enter PostgreSQL password" -AsSecureString
        $pgPasswordPlain = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($pgPassword))

        if ($pgPasswordPlain.Length -gt 0) {
            break
        } else {
            Write-Host "Password cannot be empty. Please try again." -ForegroundColor Red
        }
    } while ($true)

    Write-Host ""
    Write-Host "Updating configuration file..." -ForegroundColor Blue

    # Update docker.env file
    $content = Get-Content "docker.env"
    $content = $content -replace "COUNTRY=germany", "COUNTRY=$($selectedCountry.Name)"
    $content = $content -replace "CITYDB_SRID=25832", "CITYDB_SRID=$($selectedCountry.SRID)"
    $content = $content -replace "CITYDB_SRS_NAME=ETRS89 / UTM zone 32N", "CITYDB_SRS_NAME=$($selectedCountry.SRS)"
    $content = $content -replace "DB_USER=postgres", "DB_USER=$pgUser"
    $content = $content -replace "<your_pg_password>", $pgPasswordPlain
    $content | Set-Content "docker.env"

    Write-Host "Configuration completed!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Summary:" -ForegroundColor Cyan
    Write-Host "========" -ForegroundColor Cyan
    Write-Host "Country: $($selectedCountry.Name)" -ForegroundColor White
    Write-Host "SRID: $($selectedCountry.SRID)" -ForegroundColor White
    Write-Host "SRS Name: $($selectedCountry.SRS)" -ForegroundColor White
    Write-Host "Database: Configured" -ForegroundColor White
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "- Place your data in data\lod2\$($selectedCountry.Name)\ and data\lod3\$($selectedCountry.Name)\"
    Write-Host "- Run '.\setup.ps1 up' to start containers"
    Write-Host "- Run '.\setup.ps1 dev' to access development shell"
}

function Invoke-ConfigureManual {
    Write-Host "Manual configuration mode..." -ForegroundColor Blue
    Write-Host "Please edit docker.env manually:" -ForegroundColor Yellow
    Write-Host "   - Set COUNTRY to your target country" -ForegroundColor Yellow
    Write-Host "   - Set CITYDB_SRID to the appropriate SRID" -ForegroundColor Yellow
    Write-Host "   - Set CITYDB_SRS_NAME to the appropriate SRS name" -ForegroundColor Yellow
    Write-Host "   - Replace '<your_pg_password>' with your PostgreSQL password" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "You can edit the file with:" -ForegroundColor Cyan
    Write-Host "  notepad docker.env" -ForegroundColor White
    Write-Host "  code docker.env" -ForegroundColor White
}

function Invoke-Build {
    Write-Host "Building Docker environment..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose build --no-cache
    Set-Location ".."
}

function Invoke-Up {
    Write-Host "Starting Docker environment..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose up -d
    Set-Location ".."
}

function Invoke-Down {
    Write-Host "Stopping Docker environment..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose down
    Set-Location ".."
}

function Invoke-Dev {
    Write-Host "Starting development environment..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose up -d
    docker exec -it city2tabula-environment bash
    Set-Location ".."
}

function Invoke-CreateDb {
    Invoke-Up
    Write-Host "Creating database and setting up schemas..." -ForegroundColor Blue
    Set-Location "environment"
    docker exec -it city2tabula-environment ./city2tabula -create_db
    Set-Location ".."
}

function Invoke-ExtractFeatures {
    Invoke-Up
    Write-Host "Extracting building features..." -ForegroundColor Blue
    Set-Location "environment"
    docker exec -it city2tabula-environment ./city2tabula -extract_features
    Set-Location ".."
}

function Invoke-ResetDb {
    Invoke-Up
    Write-Host "Resetting the entire database..." -ForegroundColor Blue
    Set-Location "environment"
    docker exec -it city2tabula-environment ./city2tabula -reset_all
    Set-Location ".."
}

function Invoke-Version {
    Invoke-Up
    Write-Host "Checking City2TABULA version..." -ForegroundColor Blue
    Set-Location "environment"
    docker exec -it city2tabula-environment ./city2tabula -version
    Set-Location ".."
}

function Invoke-v {
    Invoke-Version
}

function Invoke-Setup {
    Write-Host "Setting up City2TABULA environment..." -ForegroundColor Magenta
    Invoke-Build
    Invoke-Configure
    Invoke-Up
    Write-Host "Environment is ready! Run '.\setup.ps1 dev' to access the shell" -ForegroundColor Green
}

function Invoke-QuickStart {
    Write-Host "Running complete City2TABULA pipeline..." -ForegroundColor Magenta
    Invoke-Setup
    Invoke-CreateDb
    Invoke-ExtractFeatures
    Write-Host "Quick start complete!" -ForegroundColor Green
}

function Invoke-Status {
    Write-Host "Checking container status..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose ps
    Set-Location ".."
}

function Invoke-Logs {
    Write-Host "Viewing Docker logs..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose logs -f
    Set-Location ".."
}

function Invoke-Clean {
    Write-Host "Stopping containers and removing volumes..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose down -v
    Set-Location ".."
}

function Invoke-CleanAll {
    Write-Host "Removing containers, volumes, and images..." -ForegroundColor Blue
    Set-Location "environment"
    docker compose down -v --rmi all
    Set-Location ".."
}

# Main command dispatcher
switch ($Command.ToLower()) {
    "help" { Show-Help }
    "configure" { Invoke-Configure }
    "configure-manual" { Invoke-ConfigureManual }
    "build" { Invoke-Build }
    "up" { Invoke-Up }
    "down" { Invoke-Down }
    "dev" { Invoke-Dev }
    "create-db" { Invoke-CreateDb }
    "extract-features" { Invoke-ExtractFeatures }
    "reset-db" { Invoke-ResetDb }
    "setup" { Invoke-Setup }
    "quick-start" { Invoke-QuickStart }
    "status" { Invoke-Status }
    "logs" { Invoke-Logs }
    "clean" { Invoke-Clean }
    "clean-all" { Invoke-CleanAll }
    default {
        Write-Host "Unknown command: $Command" -ForegroundColor Red
        Write-Host ""
        Show-Help
    }
}