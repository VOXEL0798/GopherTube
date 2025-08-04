#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

print_status() { echo -e "${GREEN}[INFO]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
missing_deps=()
supported_installers=(apt-get pacman dnf yum zypper brew pkg apk)

# Utility function to identify if a command exists.
command_exists() {
    if command -v "${1}" >/dev/null; then
        return 1
    fi
    return 0
}

# Determine which (if any) dependencies are missing so they can
# be selectively installed.
identify_missing_dependencies() {
    _installer="${1}"
    dependencies=(go mpv fzf chafa yt-dlp)
    case "${_installer}" in
        "apt-get"|"zypper"|"dnf"|"yum")
            dependencies+=("python3-pip")
            #break
            ;;
        "pkg")
            dependencies[${#dependencies[@]-1}]="py39-yt-dlp"
            #break
            ;;
        "apk")
            dependencies[${#dependencies[@]-1}]="py3-yt-dlp"
            #break
            ;;
    esac
    for d in "${dependencies[@]}"; do
        if command_exists "${d}"; then
            missing_deps+=("${d}")
        fi
    done
}

# Which installer should be used? Used for installing and resolving
# some package names
identify_installer() {
    for si in "${supported_installers[@]}"; do
        command -v "${si}" >/dev/null && { echo "${si}"; return; }
    done
}

# Install dependencies
install_deps() {
    installer="${1}"
    print_status "Installing missing dependencies..."
    case "${1}" in
        "apt-get")
            sudo "${installer}" update
            ;;
        "zypper"|"dnf"|"yum")
            sudo "${installer}" install -y "${missing_deps[@]}"
            pip3 install -U yt-dlp
            return
            ;;
        "pacman")
            sudo "${installer}" -S --noconfirm "${missing_deps[@]}"
            return
            ;;
        "brew")
            "${installer}" install "${missing_deps[@]}"
            return
            ;;
        "pkg")
            sudo "${installer}" install -y "${missing_deps[@]}"
            return
            ;;
        "apk")
            sudo "${installer}" add "${missing_deps[@]}"
            return
            ;;
        *)
            print_error "Unsupported package manager. Please perform manual installation."
            exit 1
            ;;
    esac
}

# Build and install
build_install() {
    print_status "Building GopherTube..."

    [ ! -d "GopherTube" ] && git clone --depth=1 https://github.com/KrishnaSSH/GopherTube.git
    cd GopherTube

    go mod download
    go build -o gophertube .

    sudo cp gophertube /usr/local/bin/
    sudo mkdir -p /usr/local/share/man/man1
    sudo cp man/gophertube.1 /usr/local/share/man/man1/ 2>/dev/null || true

    mkdir -p ~/.config/gophertube
    cp config/gophertube.toml ~/.config/gophertube/ 2>/dev/null || true

    print_status "GopherTube installed successfully!"
}

install_deps
build_install
