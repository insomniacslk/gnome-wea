#!/bin/bash
set -exu

# This is a script for systemd-suspend, to be placed in
# /usr/lib/systemd/systemd-sleep . It will be executed after the system resumes
# from suspend, and it will update the weather and current location.
#
# This script is from the wea project at https://github.com/insomniacslk/wea .

case $1 in
    pre)
        # before suspending. Do nothing
        ;;
    post)
        # after resuming. Update weather
        killall -USR1 wea
esac
