#!/usr/bin/env bash
# Bellum Linux Installer bootstrap.
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh)
#
# Downloads the latest release from this repository, checks it against the
# release's SHA256SUMS, unpacks it to ~/.local/share/bellum-installer/<version>
# and runs the guided installer. It never uses sudo on its own: if host
# packages are missing it prints the command and asks first.
#
# Options (anything else is passed to the installer):
#   --dry-run          download and verify only; do not install anything
#   --version vX.Y.Z   use a specific release instead of the latest
#   --help             show this help
#
# Environment (for testing and mirrors):
#   BELLUM_RELEASE_BASE  URL holding the release assets
#                        (default: this repository's GitHub release download URL)
#   BELLUM_VERSION       same as --version
set -euo pipefail

REPO="Ch3w3y/bellum-linux-installer"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/bellum-installer"

say()  { printf '\033[1;34m[bellum]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[bellum]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[bellum]\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF_USAGE'
Usage: install.sh [--dry-run] [--version vX.Y.Z] [installer options...]

  --dry-run          download and verify only; do not install anything
  --version vX.Y.Z   use a specific release instead of the latest
  --help             show this help

Other options are passed to the installer (try: --help after unpacking).
EOF_USAGE
}

dry_run=0
version="${BELLUM_VERSION:-}"
installer_args=()
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) dry_run=1 ;;
    --version) [ $# -ge 2 ] || die "--version needs a value, for example --version v2.2.0"; version="$2"; shift ;;
    --version=*) version="${1#--version=}" ;;
    -h|--help) usage; exit 0 ;;
    *) installer_args+=("$1") ;;
  esac
  shift
done

# --- Host checks -------------------------------------------------------------

[ "$dry_run" = 1 ] || [ "$(id -u)" -ne 0 ] || die "Don't run this as root or with sudo. Run it as your normal user; it asks before anything needs sudo."

arch="$(uname -m)"
[ "$arch" = x86_64 ] || die "Bellum needs an x86_64 (64-bit Intel or AMD) PC. This machine is $arch, which the game and Proton don't support."

os_id="" os_like="" os_variant=""
if [ -r /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  os_id="${ID:-}" os_like="${ID_LIKE:-}" os_variant="${VARIANT_ID:-}"
fi

immutable=0
case "$os_id $os_variant" in
  *steamos*|*bazzite*|*atomic*|*immutable*|*silverblue*|*kinoite*) immutable=1 ;;
esac

family=unknown
for id in $os_id $os_like; do
  case "$id" in
    arch|manjaro|endeavouros|cachyos) family=arch; break ;;
    fedora|rhel|centos|nobara) family=fedora; break ;;
    debian|ubuntu|linuxmint|pop) family=debian; break ;;
    opensuse*|suse|sles) family=opensuse; break ;;
  esac
done

have() { command -v "$1" >/dev/null 2>&1; }

for tool in tar sha256sum; do
  have "$tool" || die "'$tool' is missing. It is part of every standard Linux install; install coreutils/tar with your package manager and try again."
done
if have curl; then
  fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
  fetch_stdout() { curl -fsSL --retry 3 "$1"; }
elif have wget; then
  fetch() { wget -q -O "$2" "$1"; }
  fetch_stdout() { wget -q -O - "$1"; }
else
  die "Neither curl nor wget is installed. Install one with your package manager and try again."
fi

# Offer to install the host packages the installer needs. The Go installer
# checks again and explains anything still missing, so declining is fine.
offer_packages() {
  local missing=() pkgs=() notes=() cmd=""
  have umu-run      || missing+=(umu-run)
  have osslsigncode || missing+=(osslsigncode)
  have wget         || missing+=(wget)
  [ ${#missing[@]} -gt 0 ] || return 0

  warn "Missing tools: ${missing[*]}"
  if [ "$immutable" = 1 ]; then
    warn "This system ($os_id) is immutable. Install them through its supported method (for example a Distrobox container or the system's layering tool). Nothing will be changed automatically."
    return 0
  fi
  for tool in "${missing[@]}"; do
    case "$family:$tool" in
      arch:umu-run)      pkgs+=(umu-launcher); notes+=("umu-launcher is in the [multilib] repository, which must be enabled.") ;;
      arch:osslsigncode) notes+=("osslsigncode is in the AUR, for example: yay -S osslsigncode") ;;
      *:umu-run)         notes+=("umu-launcher: install it from https://github.com/Open-Wine-Components/umu-launcher/releases (or your distribution's repository if it has one).") ;;
      *)                 pkgs+=("$tool") ;;
    esac
  done
  case "$family" in
    arch)     cmd="sudo pacman -S --needed" ;;
    fedora)   cmd="sudo dnf install" ;;
    debian)   cmd="sudo apt install" ;;
    opensuse) cmd="sudo zypper install" ;;
  esac
  for note in "${notes[@]}"; do warn "$note"; done
  if [ ${#pkgs[@]} -eq 0 ]; then
    return 0
  fi
  if [ -z "$cmd" ]; then
    warn "Install these with your distribution's package manager: ${pkgs[*]}"
    return 0
  fi
  say "This command installs them: $cmd ${pkgs[*]}"
  if [ "$dry_run" = 1 ] || [ ! -r /dev/tty ]; then
    return 0
  fi
  local answer=""
  printf 'Run this now? [y/N] ' >/dev/tty
  read -r answer </dev/tty || answer=""
  case "$answer" in
    y|Y|yes|YES) $cmd "${pkgs[@]}" || warn "The package command failed; the installer will tell you what's still missing." ;;
    *) say "Skipped. The installer will list anything still missing." ;;
  esac
}

# --- Download and verify -------------------------------------------------------

if [ -z "$version" ]; then
  say "Looking up the latest release..."
  version="$(fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)" || true
  [ -n "$version" ] || die "Couldn't find the latest release on github.com/$REPO. Check your internet connection, or pass --version vX.Y.Z."
fi
case "$version" in
  v*) ;;
  *) version="v$version" ;;
esac
plain="${version#v}"
case "$plain" in
  ''|*[!0-9A-Za-z._-]*) die "'$version' is not a valid release version." ;;
esac

base="${BELLUM_RELEASE_BASE:-https://github.com/$REPO/releases/download/$version}"
tarball="bellum-installer-linux-amd64-$plain.tar.gz"
stem="${tarball%.tar.gz}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

say "Downloading Bellum installer $version..."
fetch "$base/$tarball" "$work/$tarball" || die "Download failed: $base/$tarball. Check your connection and that release $version exists."
fetch "$base/SHA256SUMS" "$work/SHA256SUMS" || die "Download failed: $base/SHA256SUMS."

say "Verifying checksum..."
expected="$(awk -v f="$tarball" '$2 == f || $2 == "*"f {print $1}' "$work/SHA256SUMS")"
[ -n "$expected" ] || die "SHA256SUMS has no entry for $tarball. Not installing an unverified download."
actual="$(sha256sum "$work/$tarball" | awk '{print $1}')"
[ "$expected" = "$actual" ] || die "Checksum mismatch for $tarball (expected $expected, got $actual). The download is corrupt or was tampered with; nothing was installed."

tar -xzf "$work/$tarball" -C "$work"
[ -d "$work/$stem" ] || die "The release archive doesn't contain $stem/."
(cd "$work/$stem" && sha256sum --check --strict --quiet SHA256SUMS) \
  || die "Files inside the release archive failed their checksums; nothing was installed."
say "Checksums OK."

offer_packages

if [ "$dry_run" = 1 ]; then
  say "Dry run complete: $tarball downloaded and verified. Nothing was installed or run."
  exit 0
fi

# --- Unpack and run ------------------------------------------------------------

dest="$DATA_DIR/$plain"
mkdir -p "$DATA_DIR"
rm -rf "$dest.tmp"
mv "$work/$stem" "$dest.tmp"
rm -rf "$dest"
mv "$dest.tmp" "$dest"
say "Unpacked to $dest"

rm -rf "$work"
trap - EXIT

[ -r /dev/tty ] || die "No terminal is available for the installer's questions. Run this from a terminal window."

# The installer asks questions; when this script arrives through a pipe its
# stdin is the script itself, so read answers from the terminal instead.
exec "$dest/installer" ${installer_args[@]+"${installer_args[@]}"} </dev/tty
