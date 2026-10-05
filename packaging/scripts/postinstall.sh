#!/bin/sh
# SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT
#
# Shared by the rpm (%post) and deb (postinst) packages. $1 tells them apart:
# rpm passes the instance count (1 = install, 2+ = upgrade); deb passes
# "configure", with the previously configured version in $2 on upgrade.

upgrade=
case "$1" in
    configure) [ -n "$2" ] && upgrade=1 ;;
    [0-9]*)    [ "$1" -ge 2 ] && upgrade=1 ;;
esac

# reload systemd so the new Quadlet-generated units are seen
systemctl daemon-reload || :
if [ -n "$upgrade" ]; then
    systemctl try-restart coresmd-coredhcp.service coresmd-coredns.service || :
fi
