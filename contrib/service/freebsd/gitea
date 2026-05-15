#!/bin/sh
#
# $FreeBSD$
#
# PROVIDE: gitea
# REQUIRE: NETWORKING SYSLOG
# KEYWORD: shutdown
#
# Add the following lines to /etc/rc.conf to enable gitea:
#
#gitea_enable="YES"

. /etc/rc.subr

name="gitea"
rcvar="gitea_enable"

load_rc_config $name

: ${gitea_user:="git"}
: ${gitea_enable:="NO"}
: ${gitea_directory:="/var/lib/gitea"}

command="/usr/local/bin/gitea web -c /etc/gitea/app.ini"
procname="$(echo $command |cut -d' ' -f1)"

pidfile="${gitea_directory}/${name}.pid"

start_cmd="${name}_start"
stop_cmd="${name}_stop"

gitea_start() {
	cd ${gitea_directory}
	export USER=${gitea_user}
	export HOME=/usr/home/${gitea_user}
	export GITEA_WORK_DIR=${gitea_directory}
	/usr/sbin/daemon -f -u ${gitea_user} -p ${pidfile} $command
}

gitea_stop() {
	if [ ! -f $pidfile ]; then
		echo "GITEA PID File not found. Maybe GITEA is not running?"
	else
		kill $(cat $pidfile)
	fi
}

run_rc_command "$1"
