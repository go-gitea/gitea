#!/bin/sh
# shellcheck shell=sh # Written to comply with IEEE Std 1003.1-2017

###@ Created by Jacob Hrbek identified using a GPG identifier assigned to the e-mail <kreyren@rixotstudio.cz> according to the keyserver <https://keys.openpgp.org/> under GPLv3 license <https://www.gnu.org/licenses/gpl-3.0.en.html> in 06/02/2021-EU 22:57:48 CET

###! Shell script designed to be used as an entrypoint for the gitea dockerfile

command -v die 1>/dev/null || die() { printf 'FATAL: %s\n' "$2"; exit 1 ;}

# while [ "$#" -gt 0 ]; do case "$1" in
	
# esac; shift 1; done

if [ GITEA_ROOTLESS ]; then
fi