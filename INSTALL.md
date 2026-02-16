# Installation Guide

## Quick Install

### Linux & macOS
```bash
curl -fsSL https://octrafic.com/install.sh | bash
```

### Windows (PowerShell)
```powershell
iex (iwr -useb https://octrafic.com/install.ps1)
```

---

## Update

### Using built-in updater (recommended)
```bash
octrafic --update
```

The updater automatically detects your installation method and uses the appropriate update mechanism:
- **Homebrew**: `brew upgrade octrafic`
- **yay/paru**: Updates via AUR
- **deb/rpm packages**: Re-runs installation script
- **Other installations**: Downloads and replaces binary automatically

---

## Package Managers

### macOS - Homebrew
```bash
brew install octrafic/tap/octrafic
```

### Linux - Debian/Ubuntu
```bash
# Download .deb from GitHub Releases
sudo apt install ./octrafic_VERSION_linux_amd64.deb
```

### Linux - Fedora/RHEL/CentOS
```bash
# Download .rpm from GitHub Releases
sudo dnf install octrafic_VERSION_linux_amd64.rpm
```

### Linux - Arch
```bash
yay -S octrafic-bin
# or
paru -S octrafic-bin
```

---

## Manual Installation

### Linux/macOS
1. Download the appropriate archive from [GitHub Releases](https://github.com/Octrafic/octrafic-cli/releases)
2. Extract: `tar -xzf octrafic_*.tar.gz`
3. Move binary: `sudo mv octrafic /usr/local/bin/`
4. Make executable: `sudo chmod +x /usr/local/bin/octrafic`

### Windows
1. Download `octrafic_Windows_x86_64.zip` from [GitHub Releases](https://github.com/Octrafic/octrafic-cli/releases)
2. Extract to desired location (e.g., `C:\Program Files\Octrafic`)
3. Add to PATH:
   - Open System Properties → Environment Variables
   - Edit `Path` variable
   - Add the installation directory

---

## Build from Source

Requires Go 1.26+

```bash
git clone https://github.com/Octrafic/octrafic-cli.git
cd octrafic-cli
go build -o octrafic cmd/octrafic/main.go
```

---

## Verify Installation

```bash
octrafic --help
```

---

## Uninstall

### Homebrew
```bash
brew uninstall octrafic
```

### Debian/Ubuntu
```bash
sudo apt remove octrafic
```

### Fedora/RHEL
```bash
sudo dnf remove octrafic
# or
sudo rpm -e octrafic
```

### Arch
```bash
yay -R octrafic-bin
```

### Manual Installation
```bash
# Linux/macOS
sudo rm /usr/local/bin/octrafic

# Windows (remove from PATH and delete directory)
rm -r "$env:LOCALAPPDATA\Programs\Octrafic"
```

---

## Support

- Documentation: https://docs.octrafic.com
- Issues: https://github.com/Octrafic/octrafic-cli/issues
- Email: contact@octrafic.com
