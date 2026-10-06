#!/bin/sh
# SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT
#
# Shared by the rpm (%preun) and deb (prerm) packages. Stop the services only
# on real removal: rpm passes 0, deb passes "remove" (upgrades pass 1+ /
# "upgrade").

case "$1" in
    0|remove)
        systemctl stop coresmd-coredhcp.service coresmd-coredns.service >/dev/null 2>&1 || :
        ;;
esac
