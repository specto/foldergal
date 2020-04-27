#!/bin/sh

# PROVIDE: foldergal
# REQUIRE: DAEMON NETWORKING
# KEYWORD:

. /etc/rc.subr

name="foldergal"
rcvar="foldergal_enable"

load_rc_config $name

: ${foldergal_enable:=no}
: ${foldergal_user="foldergaluser"}
: ${foldergal_chdir="/path/to/home"}

foldergal_command="/usr/local/bin/foldergal"
pidfile="/var/run/foldergal/${name}.pid"

export FOLDERGAL_PORT=8080
export FOLDERGAL_HOME="/path/to/home"
export FOLDERGAL_ROOT="/path/to/files/to/serve"
# export FOLDERGAL_PREFIX=gallery
export FOLDERGAL_CACHE_EXPIRES_AFTER=8h
# export FOLDERGAL_DISCORD_WEBHOOK="https://discordapp.com/api/webhooks/xxxxxx"
export FOLDERGAL_QUIET=true
export FOLDERGAL_HOST=0.0.0.0
export FOLDERGAL_PUBLIC_HOST=www.example.com

command="/usr/sbin/daemon"
command_args="-P ${pidfile} -r -f ${foldergal_command}"

run_rc_command "$1"
