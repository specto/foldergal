#!/bin/sh

# PROVIDE: foldergal
# REQUIRE: DAEMON NETWORKING
# KEYWORD:

. /etc/rc.subr

name="foldergal"
rcvar="foldergal_enable"

load_rc_config $name

: ${foldergal_enable:=no}
: ${foldergal_user="librarian"}
: ${foldergal_directory="/srv/foldergal"}

command="/usr/local/bin/bash"
command_args="/srv/foldergal/start.sh"

run_rc_command "$1"
