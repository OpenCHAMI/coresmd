<!--
SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# CoreSMD - Connect CoreDHCP/CoreDNS to SMD

## Contents

- [CoreSMD - Connect CoreDHCP/CoreDNS to SMD](#coresmd---connect-coredhcpcoredns-to-smd)
  - [Contents](#contents)
  - [Introduction](#introduction)
    - [CoreDHCP](#coredhcp)
    - [CoreDNS](#coredns)
  - [Overview](#overview)
  - [Configuration](#configuration)
  - [Build](#build)
    - [Build with GoReleaser](#build-with-goreleaser)
      - [Using Make Targets](#using-make-targets)
      - [Running Goreleaser Manually](#running-goreleaser-manually)
    - [Build Binaries with Make](#build-binaries-with-make)
    - [Build Container with Make](#build-container-with-make)
  - [Testing](#testing)
    - [CoreDHCP](#coredhcp-1)
    - [CoreDNS](#coredns-1)
  - [Running](#running)
    - [Configuration](#configuration-1)
    - [Preparation: SMD and BSS](#preparation-smd-and-bss)
    - [Preparation: TFTP](#preparation-tftp)
    - [Running](#running-1)
      - [CoreDHCP](#coredhcp-2)
      - [CoreDNS](#coredns-2)
  - [More Reading](#more-reading)

---

## Introduction

CoreSMD provides plugins for both [CoreDHCP](https://github.com/coredhcp/coredhcp) and [CoreDNS](https://github.com/coredns/coredns) that allow DHCP requests and DNS lookups to use [SMD](https://github.com/OpenCHAMI/smd), the OpenCHAMI inventory service.

### CoreDHCP

CoreSMD provides two plugins. The first plugin, **coresmd**, uses SMD as a source of truth to provide DHCP leases for both DHCPv4 and DHCPv6. The second plugin, **bootloop**, dynamically assigns temporary IP addresses to unknown MACs until they can be updated in SMD.

This repository is part of the [OpenCHAMI](https://openchami.org) project. It extends CoreDHCP by integrating it with the SMD service so DHCP leases can be centrally managed. There are two primary plugins:

1. **coresmd**
   Provides DHCP leases (IPv4 and IPv6) based on data from SMD.

2. **bootloop**
   Assigns temporary IPv4 addresses to unknown nodes. It also returns a DHCPNAK if it sees a node that has become known to SMD since its last lease, forcing a full DHCP handshake to get a new address (from **coresmd**).

The goal of **bootloop** is to ensure unknown nodes/BMCs continually attempt to get new IP addresses if they become known in SMD, while still having a short, discoverable address for tasks like [Magellan](https://github.com/OpenCHAMI/magellan).

See [**examples/coredhcp/**](https://github.com/OpenCHAMI/coresmd/tree/main/examples/coredhcp) for configuration examples.

### CoreDNS

The **coresmd** plugin allows hostnames/FQDNs for nodes and BMCs stored in SMD to be resolved to IP addresses (both IPv4 and IPv6). It supports A, AAAA, and PTR record lookups.

See [**examples/coredns/**](https://github.com/OpenCHAMI/coresmd/tree/main/examples/coredns) for configuration examples.

---

## Overview

CoreSMD acts as a pull-through cache of DHCP and DNS information from SMD, ensuring that new or updated details in SMD can be reflected in DHCP lease assignments and DNS records. This facilitates more dynamic environments where nodes might be added or changed frequently, and also simplifies discovery of unknown devices via the **bootloop** CoreDHCP plugin.

---

## Configuration

Take a look at [**examples/**](examples/). In there are configuration examples and documentation for both CoreDHCP and CoreDNS.

## Build

The plugins in this repository can be built into CoreDHCP/CoreDNS either using a container-based approach (via the provided Dockerfile) or by statically compiling them into CoreDHCP/CoreDNS on bare metal. Additionally, this project uses [GoReleaser](https://goreleaser.com/) to automate releases and include build metadata.

For local build options, run:

```bash
make help
```

For a list of build targets.

### Build with GoReleaser

#### Using Make Targets

If Goreleaser is already installed, the `goreleaser-*` Make targets can be used and the below steps skipped.

To build the binaries only (binaries will be located in `dist/`):

```bash
make GORELEASER_OPTS='--clean --snapshot --single-target' goreleaser-build
```

To build the containers (all supported architectures):

> [!NOTE]
> Goreleaser, as of this writing, doesn't support building a container _only_ for the native architecture. If that is desired, see [**Build a Container with Make**](#build-a-container-with-make) below.

```bash
make GORELEASER_OPTS='--clean --snapshot --skip publish' goreleaser-release
```

#### Running Goreleaser Manually

If running Goreleaser manually, ensure the following environment variables are set in order to include build metadata:

- **BUILD_HOST**: The hostname of the machine where the build is performed.
- **GO_VERSION**: The version of Go used for the build.
- **BUILD_USER**: The username of the person or system performing the build.

You can set them with:

```bash
export BUILD_HOST=$(hostname)
export GO_VERSION=$(go version | awk '{print $3}')
export BUILD_USER=$(whoami)
```

To build binaries and containers for all supported architectures:

```bash
goreleaser release --clean --snapshot --skip publish
```

To build just the binaries for the native architecture:

```bash
goreleaser build --clean --snapshot --single-target
```

Check the `dist/` directory for the built binaries, which will include the embedded metadata.

### Build Binaries with Make

Both binaries can be built with:

```
make
```

There are also Make targets for each separate binary:

```
make coredhcp
make coredns
```

These will put `coredhcp` and `coredns` binaries in the repository root which can be used for building a container in the next step.

Verify that CoreDHCP contains the **coresmd** and **bootloop** plugins:

```
$ ./coredhcp --plugins | grep -E 'coresmd|bootloop'
bootloop
coresmd
```

...and that CoreDNS contains the **coresmd** plugin:


```
$ ./coredns --plugins | grep coresmd
coresmd
```

### Build Container with Make

To build a container that contains both CoreDHCP and CoreDNS, run:

```
make container
```

> [!NOTE]
> The container runtime is Docker by default, but is configurable. For example, to use Podman:
> ```bash
> make CONTAINER_PROG="$(which podman)" container
> ```

> [!NOTE]
> Container tags are also configurable. Use `make ... CONTAINER_TAG=$TAG container` to change it.

The container contains [`dhcping`](https://github.com/nean-and-i/dhcping) that can be used as a health check for CoreDHCP.

---

## Testing

### CoreDHCP

To test the DHCP handshake, **iproute2** tools can be used to create a virtual interface in a separate network namespace that can be used to perform DORA.

```bash
# Create the network namespace called "dhcptest"
sudo ip netns add dhcptest

# Create veth pair
# veth-srv: virtual interface in parent namespace ("server")
# veth-cli: virtual interface in child namespace ("client")
sudo ip link add veth-srv type veth peer name veth-cli
sudo ip link set veth-cli netns dhcptest

# Bring up both interfaces
sudo ip link set veth-srv up
sudo ip netns exec dhcptest ip link set lo up
sudo ip netns exec dhcptest ip link set veth-cli up

# Optionally set specific MAC address on test interface
sudo ip netns exec dhcptest ip link set dev veth-cli address de:ad:c0:de:ca:fe
```

Ensure CoreDHCP can listen on **veth-srv**. If restricting the interfaces CoreDHCP listens on, ensure there's a proper entry under `listen`, e.g:

```yaml
server4:
  listen:
    - '%veth-srv'
```

Then, monitor the interface for the DORA handshake:

```bash
sudo tcpdump -nnni veth-srv -vvv 'udp port 67 or udp port 68'
```

Initiate the DHCP handshake from the namespaced virtual network interface:

```bash
# Using dhclient
sudo ip netns exec dhcptest dhclient -4 -v -d -i veth-cli

# Using BusyBox udhcpc
sudo ip netns exec dhcptest udhcpc -i veth-cli -f -vv
```

Monitor the output of `tcpdump` for the DORA handshake.

Cleaning up:

```bash
# OPTIONAL: Kill processes inside child network namespace
sudo ip netns pids dhcptest | xargs -r sudo kill

# OPTIONAL: Release lease if dhclient was used
sudo ip netns exec dhcptest dhclient -4 -r veth-cli

# Delete namespace and veth-cli interface inside it
sudo ip netns del dhcptest

# Delete parent namespace veth if it still exists
sudo ip link del veth-srv

# Ensure namespace and interfaces are gone (output should be empty)
sudo ip netns list | grep dhcptest
ip link show | grep -e veth-srv -e veth-cli
```

### CoreDNS

See: <https://github.com/OpenCHAMI/coresmd/tree/main/examples/coredns#testing>

---

## Running

### Configuration

CoreDHCP requires a config file to run. See [**examples/coredhcp/coredhcp.yaml**](examples/coredhcp-config.yaml) for an example with detailed comments on how to enable and configure **coresmd** and **bootloop**.

CoreDNS similarly has a **Corefile** to use. See [**examples/coredns/**](examples/coredns/) for examples of Corefiles.

### Preparation: SMD and BSS

Before running CoreDHCP/CoreDNS, ensure the [OpenCHAMI](https://openchami.org) services (notably **BSS** and **SMD**) are configured and running. Their URLs should match what you configure in the CoreDHCP config file.

### Preparation: TFTP

By default, **coresmd** includes a built-in TFTP server with iPXE binaries for 32-/64-bit x86/ARM (EFI) and legacy x86. If you use the **bootloop** plugin and set the iPXE boot script path to `"default"`, it will serve a built-in reboot script to unknown nodes. Alternatively, you can point this to a custom TFTP path if different functionality is desired.

### Running

Once all prerequisites are set, you can run CoreDHCP or CoreDNS.

#### CoreDHCP

- **Podman CLI**
  Use host networking and mount your config file (this example mounts in system certificate bundle):
  ```bash
  podman run \
    --rm \
    --name=coresmd-coredhcp \
    --hostname=coresmd-coredhcp \
    --cap-add=NET_ADMIN,NET_RAW \
    --volume=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem:/root_ca/root_ca.crt:ro,Z \
    --volume=/etc/openchami/configs/coredhcp.yaml:/etc/coredhcp/config.yaml:ro,Z \
    --network=host \
    ghcr.io/openchami/coresmd:latest
  ```

  > [!NOTE]
  > `--cap-add` may or may not be needed on some distros.

- **Podman Quadlet**:
  ```ini
  [Unit]
  Description=The CoreSMD CoreDHCP container

  [Container]
  ContainerName=coresmd-coredhcp

  HostName=coresmd-coredhcp
  Image=ghcr.io/openchami/coresmd:latest

  AddCapability=NET_ADMIN
  AddCapability=NET_RAW

  Volume=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem:/root_ca/root_ca.crt:ro,Z
  Volume=/etc/openchami/configs/coredhcp.yaml:/etc/coredhcp/config.yaml:ro,Z

  Network=host

  PodmanArgs=--http-proxy=false

  [Service]
  Restart=always
   ```

- **Bare Metal**
  Execute the locally built binary:
  ```bash
  ./coredhcp -conf /path/to/config.yaml
  ```

#### CoreDNS

- **Podman CLI**
  Use host networking and mount your config file (this example mounts in system certificate bundle):
  ```bash
  podman run \
    --rm \
    --name=coresmd-coredns \
    --hostname=coresmd-coredns \
    --cap-add=NET_ADMIN,NET_RAW \
    --volume=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem:/root_ca/root_ca.crt:ro,Z \
    --volume=/etc/openchami/configs/Corefile:/etc/coredhcp/Corefile:ro,Z \
    --network=host \
    ghcr.io/openchami/coresmd:latest \
    /coredns
  ```

  > [!NOTE]
  > `--cap-add` may or may not be needed on some distros.

- **Podman Quadlet**:
  ```ini
  [Unit]
  Description=The CoreSMD CoreDNS container

  [Container]
  ContainerName=coresmd-coredns

  HostName=coresmd-coredhcp
  Image=ghcr.io/openchami/coredns:latest

  Exec=/coredns

  AddCapability=NET_ADMIN
  AddCapability=NET_RAW

  Volume=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem:/root_ca/root_ca.crt:ro,Z
  Volume=/etc/openchami/configs/Corefile

  Network=host

  PodmanArgs=--http-proxy=false

  [Service]
  Restart=always
   ```

- **Bare Metal**
  Execute the locally built binary:
  ```bash
  ./cored-conf /path/to/config.yaml
  ```

---

## More Reading

- [CoreDHCP GitHub](https://github.com/coredhcp/coredhcp)
- [CoreDNS GitHub](https://github.com/coredns/coredns)
- [OpenCHAMI Project](https://openchami.org)
- [SMD GitHub](https://github.com/OpenCHAMI/smd)
- [GoReleaser Documentation](https://goreleaser.com/install/)
- [Magellan (OpenCHAMI)](https://github.com/OpenCHAMI/magellan)
