#!/bin/sh

# PROVIDE: foldergal
# REQUIRE: networking
# KEYWORD:

. /etc/rc.subr

name="foldergal"
rcvar="foldergal_enable"
command="/srv/foldergal/start.sh"
foldergal_user="librarian"
pidfile="/var/run/${name}.pid"

start_cmd="foldergal_start"
stop_cmd="foldergal_stop"
status_cmd="foldergal_status"

foldergal_start() {
        /usr/sbin/daemon -P ${pidfile} -r -f -u $foldergal_user $command
}

foldergal_stop() {
        if [ -e "${pidfile}" ]; then
                kill -s TERM `cat ${pidfile}`
        else
                echo "${name} is not running"
        fi

}

foldergal_status() {
        if [ -e "${pidfile}" ]; then
                echo "${name} is running as pid `cat ${pidfile}`"
        else
                echo "${name} is not running"
        fi
}

load_rc_config $name
: ${foldergal_enable:=no}

run_rc_command "$1"
