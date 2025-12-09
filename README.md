# CoreSMD - Connect CoreDHCP/CoreDNS to SMD

## Contents

- [CoreSMD - Connect CoreDHCP/CoreDNS to SMD](#coresmd---connect-coredhcpcoredns-to-smd)
  - [Contents](#contents)
  - [Introduction](#introduction)
    - [CoreDHCP](#coredhcp)
    - [CoreDNS](#coredns)
  - [Overview](#overview)
  - [Build and Install](#build-and-install)
    - [Build/Install with GoReleaser](#buildinstall-with-goreleaser)
      - [Environment Variables](#environment-variables)
      - [Building Locally with GoReleaser](#building-locally-with-goreleaser)
      - [Building Locally on Bare Metal](#building-locally-on-bare-metal)
      - [Building a Local Container](#building-a-local-container)
  - [Testing](#testing)
    - [CoreDHCP](#coredhcp-1)
    - [CoreDNS](#coredns-1)
  - [Running](#running)
    - [Configuration](#configuration)
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

CoreSMD provides two plugins. The first plugin, **coresmd**, uses SMD as a source of truth to provide DHCP leases. The second plugin, **bootloop**, dynamically assigns temporary IP addresses to unknown MACs until they can be updated in SMD.

This repository is part of the [OpenCHAMI](https://openchami.org) project. It extends CoreDHCP by integrating it with the SMD service so DHCP leases can be centrally managed. There are two primary plugins:

1. **coresmd**
   Provides DHCP leases based on data from SMD.

2. **bootloop**
   Assigns temporary IP addresses to unknown nodes. It also returns a DHCPNAK if it sees a node that has become known to SMD since its last lease, forcing a full DHCP handshake to get a new address (from **coresmd**).

The goal of **bootloop** is to ensure unknown nodes/BMCs continually attempt to get new IP addresses if they become known in SMD, while still having a short, discoverable address for tasks like [Magellan](https://github.com/OpenCHAMI/magellan).

See [**examples/coredhcp/**](https://github.com/OpenCHAMI/coresmd/tree/main/examples/coredhcp) for configuration examples.

### CoreDNS

The **coresmd** plugin allows hostnames/FQDNs for nodes and BMCs stored in SMD to be resolved to IP addresses.

See [**examples/coredns/**](https://github.com/OpenCHAMI/coresmd/tree/main/examples/coredns) for configuration examples.

---

## Overview

CoreSMD acts as a pull-through cache of DHCP and DNS information from SMD, ensuring that new or updated details in SMD can be reflected in DHCP lease assignments and DNS records. This facilitates more dynamic environments where nodes might be added or changed frequently, and also simplifies discovery of unknown devices via the **bootloop** CoreDHCP plugin.

---

## Build and Install

The plugins in this repository can be built into CoreDHCP/CoreDNS either using a container-based approach (via the provided Dockerfile) or by statically compiling them into CoreDHCP/CoreDNS on bare metal. Additionally, this project uses [GoReleaser](https://goreleaser.com/) to automate releases and include build metadata.

### Build/Install with GoReleaser

#### Environment Variables

To include detailed build metadata, ensure the following environment variables are set:

- **BUILD_HOST**: The hostname of the machine where the build is performed.
- **GO_VERSION**: The version of Go used for the build.
- **BUILD_USER**: The username of the person or system performing the build.

You can set them with:

```bash
export BUILD_HOST=$(hostname)
export GO_VERSION=$(go version | awk '{print $3}')
export BUILD_USER=$(whoami)
```

#### Building Locally with GoReleaser

1. [Install GoReleaser](https://goreleaser.com/install/) if not already present.
2. Run GoReleaser in snapshot mode to produce a local build without publishing:
   ```bash
   goreleaser release --snapshot --clean --skip=publish
   ```
3. Check the `dist/` directory for the built binaries, which will include the embedded metadata.

#### Building Locally on Bare Metal

Simply run `go build` for each project:

```bash
go build ./build/coredhcp
go build ./build/coredns
```

This will put `coredhcp` and `coredns` binaries in the repository root which can be used for building a container in the next step.

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

#### Building a Local Container

To build a container that contains both CoreDHCP and CoreDNS, go through the [**Building Locally on Bare Metal**](#building-locally-on-bare-metal) step above, then run:

```bash
docker build . --tag ghcr.io/openchami/coresmd:latest
```

---

## Testing

### CoreDHCP

Currently, the repository does not include standalone test scripts for these plugins. However, once compiled into CoreDHCP, you can test the overall DHCP server behavior using tools like:

- [dhcping](https://github.com/rickardw/dhcping)
- [dnsmasq DHCP client testing](http://www.thekelleys.org.uk/dnsmasq/doc.html)

Because the plugins integrate with SMD, also verify that your SMD instance is returning correct data, and confirm the environment variables (if using GoReleaser) are correctly embedded in the binary.

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

