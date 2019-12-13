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
: ${foldergal_chdir="/srv/foldergal"}

command="/srv/foldergal/_env/bin/python"
command_args="/srv/foldergal/src/www.py &"

run_rc_command "$1"
