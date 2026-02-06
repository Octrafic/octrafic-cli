
$ErrorActionPreference = "Stop"

$Repo = "Octrafic/octrafic-cli"
$BinaryName = "octrafic.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\Octrafic"

$ESC = [char]27
$script:SKY_BLUE = "$ESC[38;2;56;189;248m"
$script:CYAN = "$ESC[38;2;34;211;238m"
$script:WHITE = "$ESC[38;2;248;250;252m"
$script:YELLOW = "$ESC[38;2;253;224;71m"
$script:RED = "$ESC[38;2;251;113;133m"
$script:RESET = "$ESC[0m"

Clear-Host

Write-Host "${SKY_BLUE}░█▀█░█▀▀░▀█▀░█▀▄░█▀█░█▀▀░▀█▀░█▀▀${RESET}"
Write-Host "${SKY_BLUE}░█░█░█░░░░█░░█▀▄░█▀█░█▀▀░░█░░█░░${RESET}"
Write-Host "${SKY_BLUE}░▀▀▀░▀▀▀░░▀░░▀░▀░▀░▀░▀░░░▀▀▀░▀▀▀${RESET}"
Write-Host ""
Write-Host "${CYAN}Welcome to Octrafic Installer${RESET}"
Write-Host "${WHITE}AI-powered API testing and exploration${RESET}"
Write-Host ""
Write-Host "${YELLOW}Press Enter to begin installation...${RESET}"
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
Write-Host ""

function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "x86_64" }
        "ARM64" { return "arm64" }
        default {
            $RED = "$([char]27)[38;2;251;113;133m"
            $RESET = "$([char]27)[0m"
            Write-Host "${RED}Error: Unsupported architecture: $arch${RESET}"
            exit 1
        }
    }
}

function Get-LatestVersion {
    Write-Host "${YELLOW}Fetching latest version...${RESET}"

    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        $version = $release.tag_name -replace '^v', ''

        if ([string]::IsNullOrEmpty($version)) {
            throw "Could not determine latest version"
        }

        Write-Host "${SKY_BLUE}Latest version: v$version${RESET}"
        return $version
    }
    catch {
        Write-Host "${RED}Error fetching version: $_${RESET}"
        exit 1
    }
}

function Install-Octrafic {
    param(
        [string]$Version,
        [string]$Arch
    )

    $archiveName = "octrafic_Windows_${Arch}.zip"
    $downloadUrl = "https://github.com/$Repo/releases/download/v$Version/$archiveName"
    $tempZip = "$env:TEMP\octrafic.zip"
    $tempExtract = "$env:TEMP\octrafic_extract"

    Write-Host ""
    Write-Host "${YELLOW}Downloading Octrafic v$Version for Windows/$Arch...${RESET}"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempZip -UseBasicParsing

        if (!(Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        Write-Host "${YELLOW}Extracting...${RESET}"
        if (Test-Path $tempExtract) {
            Remove-Item -Path $tempExtract -Recurse -Force
        }
        Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force

        Write-Host "${YELLOW}Installing to $InstallDir...${RESET}"
        Move-Item -Path "$tempExtract\$BinaryName" -Destination "$InstallDir\$BinaryName" -Force

        Remove-Item -Path $tempZip -Force
        Remove-Item -Path $tempExtract -Recurse -Force

    }
    catch {
        Write-Host "${RED}Error during installation: $_${RESET}"
        exit 1
    }
}

function Add-ToPath {
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    if ($currentPath -notlike "*$InstallDir*") {
        Write-Host "${YELLOW}Adding to PATH...${RESET}"

        $newPath = $currentPath + ";" + $InstallDir
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                    [System.Environment]::GetEnvironmentVariable("Path", "User")

        Write-Host "${SKY_BLUE}Added $InstallDir to PATH${RESET}"
    }
    else {
        Write-Host "${SKY_BLUE}Already in PATH${RESET}"
    }
}

function Main {
    $arch = Get-Architecture
    $version = Get-LatestVersion

    Install-Octrafic -Version $version -Arch $arch
    Add-ToPath

    Write-Host ""
    Write-Host "${SKY_BLUE}✓ Installation complete!${RESET}"
    Write-Host ""
    Write-Host "${CYAN}Run 'octrafic --help' to get started${RESET}"
    Write-Host "${CYAN}Visit https://octrafic.com for documentation${RESET}"
    Write-Host ""
    Write-Host "${YELLOW}Note: You may need to restart your terminal for PATH changes to take effect${RESET}"
}

Main
