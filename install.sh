#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

print_status() { echo -e "${GREEN}[INFO]${NC} $*"; }
print_error() { echo -e "${RED}[ERROR]${NC} $*"; }
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

# Build gophertube program
build_gt() {
    if ! command_exists gophertube && [[ "${FORCE_BUILD:0}" -ne 1 ]]; then
        print_status "'gophertube' is already installed. Skipping build and installation."
        print_status "To force installation, set 'FORCE_BUILD' environment vairable to '1' and re-run install script."
        return 1
    fi
    print_status "Building GopherTube..."

    repo_needs_cloning=0
    # Are we in the repo?
    git_top_dir="$(git rev-parse --show-toplevel)"
    rev_parse_return_code=$?
    if [[ "${rev_parse_return_code}" -ne 0 ]]; then
        >&2 echo "Not currently in a git repository"
        repo_needs_cloning=1
    elif [[ "${PWD}" != "${git_top_dir}" ]]; then
        >&2 echo "Not in top level of git repository. Changing directory"
        cd "${git_top_dir}"
    else
        >&2 echo "In top level of git directory"
    fi
    # Is this the right repo?
    if [[  -f "go.mod" && "$(head -1 go.mod)" == "module gophertube" ]]; then
        >&2 echo "Did not find 'go.mod' file for gophertube."
        repo_needs_cloning=1
    fi
    if [[ "${repo_needs_cloning}" -eq 1 ]]; then
        if [[ -d "GopherTube" ]]; then
            cd GopherTube
            git pull https://github.com/KrishnaSSH/GopherTube.git
        else
            git clone --depth=1 https://github.com/KrishnaSSH/GopherTube.git
            cd GopherTube
        fi
    fi

    go mod download
    go build -o gophertube . && return 0
    return 1
}

# Install gophertube and man page under "${PREFIX}"
install_gt() {
    PREFIX="${PREFIX:-/usr/local}"
    print_status "Installing files under '${PREFIX}'"
    [[ 1 -gt 0 ]] && return
    sudo cp gophertube "${PREFIX}"/bin/
    sudo mkdir -p "${PREFIX}"/share/man/man1
    sudo cp man/gophertube.1 "${PREFIX}"/share/man/man1/ 2>/dev/null || true

    [[ ! -d "${HOME}"/.config/gophertube ]] && mkdir -p "${HOME}"/.config/gophertube
    [[ ! -f "${HOME}"/.config/gophertube ]] && cp config/gophertube.toml "${HOME}"/.config/gophertube/ 2>/dev/null

    print_status "GopherTube installed successfully!"
}

### BEGIN COMMAND EXECUTION
installer="$(identify_installer)"
identify_missing_dependencies "${installer}"
if [[ ${#missing_deps} -gt 0 ]]; then
  install_deps "${installer}"
fi
build_gt && install_gt
