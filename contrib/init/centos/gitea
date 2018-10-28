#!/bin/sh
#
#       /etc/rc.d/init.d/gitea
#
#       Runs the Gitea Git with a cup of tea.
#
#
# chkconfig:   - 85 15
#

### BEGIN INIT INFO
# Provides:          gitea
# Required-Start:    $remote_fs $syslog
# Required-Stop:     $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Start gitea at boot time.
# Description:       Control gitea.
### END INIT INFO

# Source function library.
. /etc/init.d/functions

# Default values

NAME=gitea
GITEA_HOME=/var/lib/${NAME}
GITEA_PATH=/usr/local/bin/${NAME}
GITEA_USER=git
SERVICENAME="Gitea - Git with a cup of tea"
LOCKFILE=/var/lock/subsys/gitea
LOGPATH=${GITEA_HOME}/log
LOGFILE=${LOGPATH}/gitea.log
RETVAL=0

# Read configuration from /etc/sysconfig/gitea to override defaults
[ -r /etc/sysconfig/$NAME ] && . /etc/sysconfig/$NAME

# Don't do anything if nothing is installed
[ -x ${GITEA_PATH} ] || exit 0
# exit if logpath dir is not created.
[ -x ${LOGPATH} ] || exit 0

DAEMON_OPTS="--check $NAME"

# Set additional options, if any
[ ! -z "$GITEA_USER" ] && DAEMON_OPTS="$DAEMON_OPTS --user=${GITEA_USER}"

start() {
  cd ${GITEA_HOME}
  echo -n "Starting ${SERVICENAME}: "
  daemon $DAEMON_OPTS "${GITEA_PATH} web -c /etc/${NAME}/app.ini > ${LOGFILE} 2>&1 &"
  RETVAL=$?
  echo
  [ $RETVAL = 0 ] && touch ${LOCKFILE}

  return $RETVAL
}

stop() {
  cd ${GITEA_HOME}
        echo -n "Shutting down ${SERVICENAME}: "
        killproc ${NAME}
        RETVAL=$?
        echo
        [ $RETVAL = 0 ] && rm -f ${LOCKFILE}
}

case "$1" in
    start)
        status ${NAME} > /dev/null 2>&1 && exit 0
        start
        ;;
    stop)
        stop
        ;;
    status)
        status ${NAME}
        ;;
    restart)
        stop
        start
        ;;
    reload)
        stop
        start
        ;;
    *)
        echo "Usage: ${NAME} {start|stop|status|restart}"
        exit 1
        ;;
esac
exit $RETVAL
