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

# export FOLDERGAL_ROOT=.
# export FOLDERGAL_HOME=.
# export FOLDERGAL_HOST=localhost
# export FOLDERGAL_PORT=8080
# export FOLDERGAL_PUBLIC_HOST=
# export FOLDERGAL_PREFIX=
# export FOLDERGAL_CACHE_EXPIRES_AFTER=0
# export FOLDERGAL_DISCORD_NAME=Gallery
# export FOLDERGAL_DISCORD_WEBHOOK=
# export FOLDERGAL_HTTP2=false
# export FOLDERGAL_NOTIFY_AFTER=30s
# export FOLDERGAL_QUIET=false
# export FOLDERGAL_THUMB_H=400
# export FOLDERGAL_THUMB_W=400
# export FOLDERGAL_TLS_CRT=
# export FOLDERGAL_TLS_KEY=
# export FOLDERGAL_FFMPEG=
# export FOLDERGAL_TIMEZONE=Local
# export FOLDERGAL_COPYRIGHT=
#
# or...
export FOLDERGAL_CONFIG="/path/to/config.json"

command="/usr/sbin/daemon"
command_args="-P ${pidfile} -r -f ${foldergal_command}"

run_rc_command "$1"
