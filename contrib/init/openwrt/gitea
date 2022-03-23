#!/bin/sh /etc/rc.common

USE_PROCD=1

# PROCD_DEBUG=1

START=90
STOP=10

PROG=/opt/gitea/gitea
GITEA_WORK_DIR=/opt/gitea
CONF_FILE=$GITEA_WORK_DIR/app.ini

start_service(){
    procd_open_instance gitea
    procd_set_param env GITEA_WORK_DIR=$GITEA_WORK_DIR
    procd_set_param env HOME=$GITEA_WORK_DIR
    procd_set_param command $PROG web -c $CONF_FILE
    procd_set_param file $CONF_FILE
    procd_set_param user git
    procd_set_param respawn ${respawn_threshold:-3600} ${respawn_timeout:-5} ${respawn_retry:-5} # respawn automatically if something died, be careful if you have an alternative process supervisor
    procd_close_instance
}

start(){
    service_start $PROG
}

stop(){
    service_stop $PROG
}

reload(){
    service_reload $PROG
}
