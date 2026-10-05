#!/bin/sh
# SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-License-Identifier: MIT
#
# Shared by the rpm (%postun) and deb (postrm) packages. Reload systemd so
# removed units are dropped; harmless on upgrades and other postrm actions.

systemctl daemon-reload || :
