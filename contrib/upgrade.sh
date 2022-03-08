#!/usr/bin/env bash
# This is an update script for gitea installed via the binary distribution
# from dl.gitea.io on linux as systemd service. It performs a backup and updates
# Gitea in place.
# NOTE: This adds the GPG Signing Key of the Gitea maintainers to the keyring.
# Depends on: bash, curl, xz, sha256sum, gpg. optionally jq.
# Usage:      [environment vars] upgrade.sh [version]
#   See section below for available environment vars.
#   When no version is specified, updates to the latest release.
# Examples:
#   upgrade.sh 1.15.10
#   giteahome=/opt/gitea giteaconf=$giteahome/app.ini upgrade.sh

while true; do
  case "$1" in
    -v | --version ) ver="$2"; shift 2 ;;
    -y | --yes ) no_confirm="yes"; shift ;;
    --ignore-gpg) ignore_gpg="yes"; shift ;;
    -- ) shift; break ;;
    * ) break ;;
  esac
done

set -euo pipefail


function require {
  for exe in "$@"; do
    command -v "$exe" &>/dev/null || (echo "missing dependency '$exe'"; exit 1)
  done
}


require curl xz sha256sum gpg

if [[ -f /etc/os-release ]]; then
  os_release=$(cat /etc/os-release)

  if [[ "$os_release" =~ "OpenWrt" ]]; then
    sudocmd="su"
    service_start="/etc/init.d/gitea start"
    service_stop="/etc/init.d/gitea stop"
    service_status="/etc/init.d/gitea status"
  else
    require systemctl
  fi
fi


# apply variables from environment
: "${giteabin:="/usr/local/bin/gitea"}"
: "${giteahome:="/var/lib/gitea"}"
: "${giteaconf:="/etc/gitea/app.ini"}"
: "${giteauser:="git"}"
: "${sudocmd:="sudo"}"
: "${arch:="linux-amd64"}"
: "${service_start:="$sudocmd systemctl start gitea"}"
: "${service_stop:="$sudocmd systemctl stop gitea"}"
: "${service_status:="$sudocmd systemctl status gitea"}"
: "${backupopts:=""}" # see `gitea dump --help` for available options


function giteacmd {
  if [[ $sudocmd = "su" ]]; then
    "$sudocmd" - "$giteauser" -c "$giteabin" --config "$giteaconf" --work-path "$giteahome" "$@"
  else
    "$sudocmd" --user "$giteauser" "$giteabin" --config "$giteaconf" --work-path "$giteahome" "$@"
  fi
}

# select version to install
if [[ -z "${ver:-}" ]]; then
  require jq
  giteaversion=$(curl --connect-timeout 10 -sL https://dl.gitea.io/gitea/version.json | jq -r .latest.version)
else
  giteaversion="$ver"
fi


# confirm update
current=$(giteacmd --version | cut -d ' ' -f 3)
[[ "$current" == "$giteaversion" ]] && echo "$current is already installed, stopping." && exit 1
if [[ -z "${no_confirm:-}"  ]]; then
  echo "Make sure to read the changelog first: https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md"
  echo "Are you ready to update Gitea from ${current} to ${giteaversion}? (y/N)"
  read -r confirm
  [[ "$confirm" == "y" ]] || [[ "$confirm" == "Y" ]] || exit 1
fi

echo "Upgrading gitea from $current to $giteaversion ..."

pushd "$(pwd)" &>/dev/null
cd "$giteahome" # needed for gitea dump later

# download new binary
binname="gitea-${giteaversion}-${arch}"
binurl="https://dl.gitea.io/gitea/${giteaversion}/${binname}.xz"
echo "Downloading $binurl..."
curl --connect-timeout 10 --silent --show-error --fail --location -O "$binurl{,.sha256,.asc}"

# validate checksum & gpg signature (exit script if error)
sha256sum -c "${binname}.xz.sha256"
if [[ -z "${ignore_gpg:-}" ]]; then
  gpg --keyserver keys.openpgp.org --recv 7C9E68152594688862D62AF62D9AE806EC1592E2
  gpg --verify "${binname}.xz.asc" "${binname}.xz" || { echo 'Signature does not match'; exit 1; }
fi
rm "${binname}".xz.{sha256,asc}

# unpack binary + make executable
xz --decompress "${binname}.xz"
chown "$giteauser" "$binname"
chmod +x "$binname"

# stop gitea, create backup, replace binary, restart gitea
echo "Stopping gitea at $(date)"
giteacmd manager flush-queues
$service_stop
echo "Creating backup in $giteahome"
giteacmd dump $backupopts
echo "Updating binary at $giteabin"
cp -f "$giteabin" "$giteabin.bak" && mv -f "$binname" "$giteabin"
$service_start
$service_status

echo "Upgrade to $giteaversion successful!"

popd
