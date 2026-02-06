#!/bin/bash

set -e

SKY_BLUE='\033[38;2;56;189;248m'
SKY_BLUE_DARK='\033[38;2;14;165;233m'
CYAN='\033[38;2;34;211;238m'
YELLOW='\033[38;2;253;224;71m'
WHITE='\033[38;2;248;250;252m'
RED='\033[38;2;251;113;133m'
NC='\033[0m'

REPO="Octrafic/octrafic-cli"
BINARY_NAME="octrafic"
INSTALL_DIR="/usr/local/bin"
DRY_RUN=false

for arg in "$@"; do
    case $arg in
        --dry-run|--test)
            DRY_RUN=true
            ;;
    esac
done

clear

echo -e "${SKY_BLUE}░█▀█░█▀▀░▀█▀░█▀▄░█▀█░█▀▀░▀█▀░█▀▀${NC}"
echo -e "${SKY_BLUE}░█░█░█░░░░█░░█▀▄░█▀█░█▀▀░░█░░█░░${NC}"
echo -e "${SKY_BLUE}░▀▀▀░▀▀▀░░▀░░▀░▀░▀░▀░▀░░░▀▀▀░▀▀▀${NC}"
echo
echo -e "${CYAN}Welcome to Octrafic Installer${NC}"
echo -e "${WHITE}AI-powered API testing and exploration${NC}"
echo

if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}[DRY RUN MODE - No actual changes will be made]${NC}"
    echo
else
    echo -e "${YELLOW}Press Enter to begin installation...${NC}"
    read -r
    echo
fi

run_cmd() {
    if [ "$DRY_RUN" = true ]; then
        echo -e "${CYAN}[DRY RUN] Would run: $*${NC}"
    else
        "$@"
    fi
}

detect_os() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$OS" in
        linux*)  OS='linux' ;;
        darwin*) OS='darwin' ;;
        *)
            echo -e "${RED}Error: Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac
}

detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH='amd64' ;;
        aarch64) ARCH='arm64' ;;
        arm64)   ARCH='arm64' ;;
        armv7l)  ARCH='armv7' ;;
        armv6l)  ARCH='armv7' ;;
        arm*)    ARCH='armv7' ;;
        *)
            echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac
}

get_latest_version() {
    echo "Fetching latest version..."
    RELEASE_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/' | tr -d '[:space:]')

    if [ -z "$RELEASE_VERSION" ]; then
        echo -e "${RED}Error: Could not determine latest version${NC}"
        exit 1
    fi

    echo -e "${SKY_BLUE}Latest version: v$RELEASE_VERSION${NC}"
}

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    elif [ -f /etc/lsb-release ]; then
        . /etc/lsb-release
        DISTRO=$DISTRIB_ID
    else
        DISTRO="unknown"
    fi
}

install_linux_package() {
    detect_distro

    case "$DISTRO" in
        ubuntu|debian|pop|linuxmint)
            echo -e "${YELLOW}Detected Debian-based distribution${NC}"
            PACKAGE_URL="https://github.com/$REPO/releases/download/v$RELEASE_VERSION/${BINARY_NAME}_${RELEASE_VERSION}_${OS}_${ARCH}.deb"
            TEMP_DEB="/tmp/${BINARY_NAME}.deb"

            echo "Downloading .deb package..."
            echo "URL: $PACKAGE_URL"
            curl -fsSL "$PACKAGE_URL" -o "$TEMP_DEB"

            echo "Installing (requires sudo)..."
            sudo dpkg -i "$TEMP_DEB" || sudo apt-get install -f -y
            rm -f "$TEMP_DEB"
            ;;

        fedora|rhel|centos|rocky|almalinux)
            echo -e "${YELLOW}Detected RedHat-based distribution${NC}"
            PACKAGE_URL="https://github.com/$REPO/releases/download/v$RELEASE_VERSION/${BINARY_NAME}_${RELEASE_VERSION}_${OS}_${ARCH}.rpm"
            TEMP_RPM="/tmp/${BINARY_NAME}.rpm"

            echo "Downloading .rpm package..."
            echo "URL: $PACKAGE_URL"
            curl -fsSL "$PACKAGE_URL" -o "$TEMP_RPM"

            echo "Installing (requires sudo)..."
            if command -v dnf &> /dev/null; then
                sudo dnf install -y "$TEMP_RPM"
            else
                sudo rpm -i "$TEMP_RPM"
            fi
            rm -f "$TEMP_RPM"
            ;;

        arch|manjaro)
            echo -e "${YELLOW}Detected Arch-based distribution${NC}"
            echo -e "${SKY_BLUE}For Arch Linux, please use AUR:${NC}"
            echo "  yay -S octrafic-bin"
            echo "  or"
            echo "  paru -S octrafic-bin"
            echo
            echo "Falling back to binary installation..."
            install_binary
            ;;

        *)
            echo -e "${YELLOW}Unknown distribution, installing binary directly${NC}"
            install_binary
            ;;
    esac
}

install_binary() {
    ARCHIVE_EXT="tar.gz"

    if [ "$OS" = "linux" ]; then
        case "$ARCH" in
            amd64)  ARCHIVE_NAME="${BINARY_NAME}_Linux_x86_64.${ARCHIVE_EXT}" ;;
            arm64)  ARCHIVE_NAME="${BINARY_NAME}_Linux_arm64.${ARCHIVE_EXT}" ;;
            armv7)  ARCHIVE_NAME="${BINARY_NAME}_Linux_armv7.${ARCHIVE_EXT}" ;;
        esac
    else
        case "$ARCH" in
            amd64)  ARCHIVE_NAME="${BINARY_NAME}_Darwin_x86_64.${ARCHIVE_EXT}" ;;
            arm64)  ARCHIVE_NAME="${BINARY_NAME}_Darwin_arm64.${ARCHIVE_EXT}" ;;
        esac
    fi

    DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$RELEASE_VERSION/$ARCHIVE_NAME"
    TEMP_DIR=$(mktemp -d)
    TEMP_ARCHIVE="$TEMP_DIR/${BINARY_NAME}.${ARCHIVE_EXT}"

    echo "Downloading $BINARY_NAME..."
    curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_ARCHIVE"

    echo "Extracting..."
    tar -xzf "$TEMP_ARCHIVE" -C "$TEMP_DIR"

    echo "Installing to $INSTALL_DIR (requires sudo)..."
    sudo mv "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"

    rm -rf "$TEMP_DIR"
}

main() {
    detect_os
    detect_arch
    get_latest_version

    echo
    echo "Installing Octrafic v$RELEASE_VERSION for $OS/$ARCH..."
    echo

    if [ "$OS" = "linux" ]; then
        install_linux_package
    else
        # macOS - use Homebrew if available, otherwise binary
        if command -v brew &> /dev/null; then
            echo -e "${YELLOW}Homebrew detected. We recommend:${NC}"
            echo "  brew install octrafic/tap/octrafic"
            echo
            read -p "Install via Homebrew? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                brew install octrafic/tap/octrafic
            else
                install_binary
            fi
        else
            install_binary
        fi
    fi

    echo
    echo -e "${SKY_BLUE}✓ Installation complete!${NC}"
    echo
    echo "Run 'octrafic --help' to get started"
    echo "Visit https://octrafic.com for documentation"
}

main
