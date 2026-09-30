# SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
# SPDX-License-Identifier: MIT
#
# See `make rpm-build` and docs/RPM_PACKAGING.md for the tag-to-version
# mapping and how the packaged quadlet's image tag is pinned to it.

Name:           coresmd-quadlet
Version:        %{version}
Release:        %{rel}%{?dist}
Summary:        OpenCHAMI CoreSMD Quadlet units

License:        MIT
URL:            https://github.com/OpenCHAMI/coresmd
Source0:        %{name}-%{version}.tar.gz

BuildArch:      noarch

Requires(post,preun,postun):  systemd
Requires:                     podman >= 5.0.0

Suggests:                     smd-quadlet >= 2.20.0
Suggests:                     tokensmith-quadlet >= 0.4.0
Suggests:                     openchami-haproxy-quadlet >= 0.0.1

%description
Podman Quadlet unit files for running CoreSMD as part of an OpenCHAMI
deployment.

%prep
%setup -q

%install

# systemd files
install -d %{buildroot}/usr/share/containers/systemd
for f in coresmd-coredhcp coresmd-coredns; do
    grep -q '@IMAGE_TAG@' $f.container
    sed "s|@IMAGE_TAG@|v%{version}|" $f.container \
        | install -m 644 /dev/stdin %{buildroot}/usr/share/containers/systemd/$f.container
    install -d %{buildroot}/usr/share/containers/systemd/$f.container.d
    install -m 644 $f.container.d/10-defaults-service.conf \
        %{buildroot}/usr/share/containers/systemd/$f.container.d/
done

install -d %{buildroot}/usr/share/containers/systemd/coresmd-.container.d
install -m 644 coresmd-.container.d/10-defaults-shared.conf \
    %{buildroot}/usr/share/containers/systemd/coresmd-.container.d/

# configuration files
install -d %{buildroot}/etc/openchami/configs
install -m 644 Corefile %{buildroot}/etc/openchami/configs/
install -m 644 coredhcp.yaml %{buildroot}/etc/openchami/configs/

%files
%license LICENSES/MIT.txt
%dir /etc/openchami
%dir /etc/openchami/configs
%config(noreplace) /etc/openchami/configs/coredhcp.yaml
%config(noreplace) /etc/openchami/configs/Corefile
/usr/share/containers/systemd/coresmd-.container.d
/usr/share/containers/systemd/coresmd-.container.d/10-defaults-shared.conf
/usr/share/containers/systemd/coresmd-coredhcp.container
/usr/share/containers/systemd/coresmd-coredhcp.container.d
/usr/share/containers/systemd/coresmd-coredhcp.container.d/10-defaults-service.conf
/usr/share/containers/systemd/coresmd-coredns.container
/usr/share/containers/systemd/coresmd-coredns.container.d
/usr/share/containers/systemd/coresmd-coredns.container.d/10-defaults-service.conf

%post
# reload systemd so the new Quadlet-generated unit is seen
systemctl daemon-reload || :
if [ $1 -ge 2 ]; then
    systemctl try-restart coresmd-coredhcp.service || :
    systemctl try-restart coresmd-coredns.service || :
fi

%preun
if [ $1 -eq 0 ]; then
    systemctl stop coresmd-coredhcp.service >/dev/null 2>&1 || :
    systemctl stop coresmd-coredns.service >/dev/null 2>&1 || :
fi

%postun
# reload systemd so the removed unit is dropped
systemctl daemon-reload || :
