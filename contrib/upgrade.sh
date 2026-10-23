#!/usr/bin/env bash
# This is an update script for codenest installed via the binary distribution
# from dl.gitea.com on linux as systemd service. It performs a backup and updates
# CodeNest in place.
# NOTE: This adds the GPG Signing Key of the CodeNest maintainers to the keyring.
# Depends on: bash, curl, xz, sha256sum. optionally jq, gpg
#   See section below for available environment vars.
#   When no version is specified, updates to the latest release.
# Examples:
#   upgrade.sh 1.15.10
#   codenesthome=/opt/codenest codenestconf=$codenesthome/app.ini upgrade.sh

# Check if codenest service is running
if ! pidof codenest &> /dev/null; then
  echo "Error: codenest is not running."
  exit 1
fi

# Continue with rest of the script if codenest is running
echo "CodeNest is running. Continuing with rest of script..."

# apply variables from environment
: "${codenestbin:="/usr/local/bin/codenest"}"
: "${codenesthome:="/var/lib/codenest"}"
: "${codenestconf:="/etc/codenest/app.ini"}"
: "${codenestuser:="git"}"
: "${sudocmd:="sudo"}"
: "${arch:="linux-amd64"}"
: "${service_start:="$sudocmd systemctl start codenest"}"
: "${service_stop:="$sudocmd systemctl stop codenest"}"
: "${service_status:="$sudocmd systemctl status codenest"}"
: "${backupopts:=""}" # see `codenest dump --help` for available options

function codenestcmd {
  if [[ $sudocmd = "su" ]]; then
    # `-c` only accept one string as argument.
    "$sudocmd" - "$codenestuser" -c "$(printf "%q " "$codenestbin" "--config" "$codenestconf" "--work-path" "$codenesthome" "$@")"
  else
    "$sudocmd" --user "$codenestuser" "$codenestbin" --config "$codenestconf" --work-path "$codenesthome" "$@"
  fi
}

function require {
  for exe in "$@"; do
    command -v "$exe" &>/dev/null || (echo "missing dependency '$exe'"; exit 1)
  done
}

# parse command line arguments
while true; do
  case "$1" in
    -v | --version ) codenestversion="$2"; shift 2 ;;
    -y | --yes ) no_confirm="yes"; shift ;;
    --ignore-gpg) ignore_gpg="yes"; shift ;;
    "" | -- ) shift; break ;;
    * ) echo "Usage:  [<environment vars>] upgrade.sh [-v <version>] [-y] [--ignore-gpg]"; exit 1;; 
  esac
done

# exit once any command fails. this means that each step should be idempotent!
set -euo pipefail

if [[ -f /etc/os-release ]]; then
  os_release=$(cat /etc/os-release)

  if [[ "$os_release" =~ "OpenWrt" ]]; then
    sudocmd="su"
    service_start="/etc/init.d/codenest start"
    service_stop="/etc/init.d/codenest stop"
    service_status="/etc/init.d/codenest status"
  else
    require systemctl
  fi
fi

require curl xz sha256sum "$sudocmd"

# select version to install
if [[ -z "${codenestversion:-}" ]]; then
  require jq
  codenestversion=$(curl --connect-timeout 10 -sL https://dl.gitea.com/gitea/version.json | jq -r .latest.version)
  echo "Latest available version is $codenestversion"
fi

# confirm update
echo "Checking currently installed version..."
current=$(codenestcmd --version | cut -d ' ' -f 3)
[[ "$current" == "$codenestversion" ]] && echo "$current is already installed, stopping." && exit 0
if [[ -z "${no_confirm:-}"  ]]; then
  echo "Make sure to read the changelog first: https://github.com/go-gitea/gitea/releases"
  echo "Are you ready to update CodeNest from ${current} to ${codenestversion}? (y/N)"
  read -r confirm
  [[ "$confirm" == "y" ]] || [[ "$confirm" == "Y" ]] || exit 1
fi

echo "Upgrading codenest from $current to $codenestversion ..."

pushd "$(pwd)" &>/dev/null
cd "$codenesthome" # needed for codenest dump later

# download new binary
binname="codenest-${codenestversion}-${arch}"
binurl="https://dl.gitea.com/gitea/${codenestversion}/${binname}.xz"
echo "Downloading $binurl..."
curl --connect-timeout 10 --silent --show-error --fail --location -O "$binurl{,.sha256,.asc}"

# validate checksum & gpg signature
sha256sum -c "${binname}.xz.sha256"
if [[ -z "${ignore_gpg:-}" ]]; then
  require gpg
  # try to use curl first, it uses standard tcp 443 port and works better behind strict firewall rules
  curl -fsSL --connect-timeout 10 "https://keys.openpgp.org/vks/v1/by-fingerprint/7C9E68152594688862D62AF62D9AE806EC1592E2" | gpg --import \
    || gpg --keyserver keys.openpgp.org --recv 7C9E68152594688862D62AF62D9AE806EC1592E2
  gpg --verify "${binname}.xz.asc" "${binname}.xz" || { echo 'Signature does not match'; exit 1; }
fi
rm "${binname}".xz.{sha256,asc}

# unpack binary + make executable
xz --decompress --force "${binname}.xz"
chown "$codenestuser" "$binname"
chmod +x "$binname"

# stop codenest, create backup, replace binary, restart codenest
echo "Flushing codenest queues at $(date)"
codenestcmd manager flush-queues
echo "Stopping codenest at $(date)"
$service_stop
echo "Creating backup in $codenesthome"
# shellcheck disable=SC2086 # flag string
codenestcmd dump $backupopts
echo "Updating binary at $codenestbin"
cp -f "$codenestbin" "$codenestbin.bak" && mv -f "$binname" "$codenestbin"
# Restore SELinux context if applicable (e.g. RHEL/Fedora)
command -v restorecon &>/dev/null && restorecon -v "$codenestbin" || true
$service_start
$service_status

echo "Upgrade to $codenestversion successful!"

popd
