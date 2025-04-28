#!/bin/sh
#
# $OpenBSD$

daemon="/usr/local/bin/gitea"
daemon_user="git"
daemon_flags="web -c /etc/gitea/app.ini"

gitea_directory="/var/lib/gitea"

rc_bg=YES

. /etc/rc.d/rc.subr

rc_start() {
	${rcexec} "cd ${gitea_directory}; ${daemon} ${daemon_flags} ${_bg}"
}

rc_cmd $1
