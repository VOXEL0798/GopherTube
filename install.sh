#!/usr/bin/env sh

set -eu

# --------------------------------------
# GopherTube Installer (POSIX)
# --------------------------------------

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC="\033[0m"

log()  { printf "%b\n" "${GREEN}[INFO]${NC} $*"; }
warn() { printf "%b\n" "${YELLOW}[WARN]${NC} $*"; }
err()  { printf "%b\n" "${RED}[ERROR]${NC} $*"; }

PREFIX=${PREFIX:-/usr/local}

has_cmd() { command -v "$1" >/dev/null 2>&1; }

detect_pm() {
  for pm in apt-get apt dnf yum zypper pacman apk brew pkg; do
    if has_cmd "$pm"; then echo "$pm"; return 0; fi
  done
  err "No supported package manager found. Please install dependencies manually."; exit 1
}

# Install a single package name with the chosen PM
pm_install_one() {
  pm="$1"; pkg="$2"
  case "$pm" in
    apt-get|apt)
      sudo "$pm" update -y >/dev/null 2>&1 || true
      sudo "$pm" install -y "$pkg"
      ;;
    dnf|yum)
      sudo "$pm" install -y "$pkg"
      ;;
    zypper)
      sudo zypper install -y "$pkg"
      ;;
    pacman)
      sudo pacman -S --noconfirm --needed "$pkg"
      ;;
    apk)
      sudo apk add --no-progress "$pkg"
      ;;
    brew)
      brew install "$pkg"
      ;;
    pkg)
      sudo pkg install -y "$pkg"
      ;;
    *)
      err "Unsupported package manager: $pm"; exit 1
      ;;
  esac
}

# ensure_cmd <cmd> <pm> <apt_pkg> <dnf_pkg> <pacman_pkg> <zypper_pkg> <apk_pkg> <brew_pkg> <pkg_pkg>
ensure_cmd() {
  cmd="$1"; pm="$2"; apt_pkg="$3"; dnf_pkg="$4"; pacman_pkg="$5"; zypper_pkg="$6"; apk_pkg="$7"; brew_pkg="$8"; pkg_pkg="$9"
  if has_cmd "$cmd"; then
    log "$cmd already present. Skipping installation."
    return 0
  fi
  pkg=""
  case "$pm" in
    apt-get|apt) pkg="$apt_pkg" ;;
    dnf|yum)     pkg="$dnf_pkg" ;;
    pacman)      pkg="$pacman_pkg" ;;
    zypper)      pkg="$zypper_pkg" ;;
    apk)         pkg="$apk_pkg" ;;
    brew)        pkg="$brew_pkg" ;;
    pkg)         pkg="$pkg_pkg" ;;
  esac
  if [ -n "$pkg" ]; then
    log "Installing missing dependency: $cmd ($pkg)"
    pm_install_one "$pm" "$pkg"
  else
    warn "No package mapping for $cmd on $pm. Please install it manually."
  fi
}

ensure_yt_dlp() {
  pm="$1"
  if has_cmd yt-dlp; then
    log "yt-dlp already present. Skipping installation."
    return 0
  fi
  # Try native package first
  case "$pm" in
    apt-get|apt|dnf|yum|zypper|pacman|apk|brew)
      log "Installing yt-dlp via package manager"
      pm_install_one "$pm" yt-dlp || true
      ;;
    pkg)
      pm_install_one "$pm" py39-yt-dlp || true
      ;;
  esac
  if has_cmd yt-dlp; then return 0; fi
  # Fallback to pip (user install). Do not upgrade if present; only install if missing.
  if ! has_cmd pip3; then
    log "pip3 not found. Installing minimal pip3..."
    case "$pm" in
      apt-get|apt) pm_install_one "$pm" python3-pip;;
      dnf|yum)     pm_install_one "$pm" python3-pip;;
      zypper)      pm_install_one "$pm" python3-pip;;
      apk)         pm_install_one "$pm" py3-pip;;
      pacman)      pm_install_one "$pm" python-pip;;
      brew)        warn "Install pip3 manually or ensure yt-dlp via brew.";;
      pkg)         pm_install_one "$pm" py39-pip || pm_install_one "$pm" python3;;
    esac
  fi
  if has_cmd pip3; then
    log "Installing yt-dlp via pip3 (user)"
    pip3 install --user yt-dlp || true
    hash -r 2>/dev/null || true
  else
    warn "pip3 not available. Please install yt-dlp manually."
  fi
}

main() {
  pm=`detect_pm`
  log "Detected package manager: $pm"

  # Ensure dependencies ONLY if missing
  ensure_cmd go    "$pm" golang-go go go go go go go
  ensure_cmd git   "$pm" git git git git git git git
  ensure_cmd mpv   "$pm" mpv mpv mpv mpv mpv mpv mpv
  ensure_cmd fzf   "$pm" fzf fzf fzf fzf fzf fzf fzf
  ensure_cmd chafa "$pm" chafa chafa chafa chafa chafa chafa chafa
  ensure_yt_dlp "$pm"

  # Determine latest version tag from GitHub
  REPO_URL="https://github.com/KrishnaSSH/GopherTube.git"
  LATEST_TAG=`git ls-remote --tags --refs "$REPO_URL" 2>/dev/null | awk -F/ '{print $NF}' | sort -V | tail -1`
  if [ -z "$LATEST_TAG" ]; then
    warn "Could not determine latest tag; falling back to 'main'"
    LATEST_TAG="main"
  fi

  # If already installed at latest, skip rebuild
  if has_cmd gophertube; then
    INSTALLED_VER=`gophertube --version 2>/dev/null | grep -oE 'v[0-9]+(\.[0-9]+)*' | head -1 || true`
    if [ -n "$INSTALLED_VER" ] && [ "$LATEST_TAG" = "$INSTALLED_VER" ]; then
      log "Already on latest version ($INSTALLED_VER). Nothing to do."
      exit 0
    fi
  fi

  # Build latest in a temp directory
  TMPDIR=`mktemp -d 2>/dev/null || mktemp -d -t gophertube`
  trap 'rm -rf "$TMPDIR"' EXIT INT TERM
  log "Building GopherTube (version: $LATEST_TAG)..."
  export GIT_TERMINAL_PROMPT=0
  git clone --depth=1 --branch "$LATEST_TAG" "$REPO_URL" "$TMPDIR/GopherTube" >/dev/null 2>&1 || {
    warn "Clone failed; trying default branch"
    git clone --depth=1 "$REPO_URL" "$TMPDIR/GopherTube" >/dev/null 2>&1 || { err "Failed to clone repository"; exit 1; }
  }
  (
    cd "$TMPDIR/GopherTube"
    if [ -f go.mod ]; then
      go mod download
    fi
    # Embed version into binary when tag is semver; if not, use 'dev'
    VFLAG="-X gophertube/internal/app.version=$LATEST_TAG"
    case "$LATEST_TAG" in
      v[0-9]*) : ;; # keep tag
      *) VFLAG="-X gophertube/internal/app.version=dev" ;;
    esac
    go build -ldflags "$VFLAG" -o gophertube ./
  )

  # Install
  log "Installing to ${PREFIX}"
  sudo mkdir -p "${PREFIX}/bin"
  sudo cp -f "$TMPDIR/GopherTube/gophertube" "${PREFIX}/bin/"
  sudo chmod +x "${PREFIX}/bin/gophertube"

  # Man page (optional)
  if [ -f "$TMPDIR/GopherTube/man/gophertube.1" ]; then
    sudo mkdir -p "${PREFIX}/share/man/man1"
    sudo cp -f "$TMPDIR/GopherTube/man/gophertube.1" "${PREFIX}/share/man/man1/"
  fi

  # User config bootstrap (do not overwrite)
  cfg_dir="$HOME/.config/gophertube"
  cfg_file="$cfg_dir/gophertube.toml"
  mkdir -p "$cfg_dir"
  if [ ! -f "$cfg_file" ] && [ -f "$TMPDIR/GopherTube/config/gophertube.toml" ]; then
    cp "$TMPDIR/GopherTube/config/gophertube.toml" "$cfg_dir/"
  fi

  log "GopherTube installed successfully. Run: gophertube"
}

main "$@"
