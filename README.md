
# CoreSMD

## Summary of Repo

CoreSMD provides two plugins for [CoreDHCP](https://github.com/coredhcp/coredhcp) that integrate with [SMD](https://github.com/OpenCHAMI/smd). The first plugin, **coresmd**, uses SMD as a source of truth to provide DHCP leases. The second plugin, **bootloop**, dynamically assigns temporary IP addresses to unknown MACs until they can be updated in SMD.

---

## Table of Contents

- [About / Introduction](#about--introduction)
- [Overview](#overview)
- [Build / Install](#build--install)
  - [Build/Install with GoReleaser](#buildinstall-with-goreleaser)
    - [Environment Variables](#environment-variables)
    - [Building Locally with GoReleaser](#building-locally-with-goreleaser)
    - [Container](#container)
    - [Bare Metal](#bare-metal)
- [Testing](#testing)
- [Running](#running)
  - [Configuration](#configuration)
  - [Preparation: SMD and BSS](#preparation-smd-and-bss)
  - [Preparation: TFTP](#preparation-tftp)
  - [Running CoreDHCP](#running-coredhcp)
- [More Reading](#more-reading)

---

## About / Introduction

This repository is part of the [OpenCHAMI](https://openchami.org) project. It extends CoreDHCP by integrating it with the SMD service so DHCP leases can be centrally managed. There are two primary plugins:

1. **coresmd**  
   Provides DHCP leases based on data from SMD.

2. **bootloop**  
   Assigns temporary IP addresses to unknown nodes. It also returns a DHCPNAK if it sees a node that has become known to SMD since its last lease, forcing a full DHCP handshake to get a new address (from **coresmd**).

The goal of **bootloop** is to ensure unknown nodes/BMCs continually attempt to get new IP addresses if they become known in SMD, while still having a short, discoverable address for tasks like [Magellan](https://github.com/OpenCHAMI/magellan).

---

## Overview

CoreSMD acts as a pull-through cache of DHCP information from SMD, ensuring that new or updated details in SMD can be reflected in DHCP lease assignments. This facilitates more dynamic environments where nodes might be added or changed frequently, and also simplifies discovery of unknown devices via the **bootloop** plugin.

---

## Build / Install

The plugins in this repository can be built into CoreDHCP either using a container-based approach (via the provided Dockerfile) or by statically compiling them into CoreDHCP on bare metal. Additionally, this project uses [GoReleaser](https://goreleaser.com/) to automate releases and include build metadata.

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
   goreleaser release --snapshot --skip-publish --clean
   ```
3. Check the `dist/` directory for the built binaries, which will include the embedded metadata.

#### Container

This repository includes a `Dockerfile` that builds CoreDHCP (with its core plugins) plus **coresmd** and **bootloop**:

```bash
docker build . --tag ghcr.io/openchami/coresmd:latest
```

#### Bare Metal

> [!NOTE]
> Certain source files in CoreDHCP only build on Linux. This may cause build errors on other platforms (e.g., macOS).  

**Prerequisites**  
- go >= 1.21  
- git  
- bash  
- sed  

1. Create a clean directory for build artifacts:
   ```bash
   mkdir build
   ```
2. Clone CoreSMD (this is **not strictly** required for building, but **is** needed if you want the plugin version included):
   ```bash
   git clone https://github.com/OpenCHAMI/coresmd
   cd coresmd
   ./gen_version.bash
   cd ..
   ```
3. Clone CoreDHCP and switch to the `coredhcp-generator` directory:
   ```bash
   git clone https://github.com/coredhcp/coredhcp
   cd coredhcp/cmds/coredhcp-generator
   go mod download
   go build
   ```
4. Run the generator to produce CoreDHCP with **coresmd** and **bootloop**:
   ```bash
   ./coredhcp-generator \
     -f core-plugins.txt \
     -t coredhcp.go.template \
     -o ../../../build/coredhcp.go \
     github.com/OpenCHAMI/coresmd/coresmd \
     github.com/OpenCHAMI/coresmd/bootloop
   ```
5. Initialize the build directory as a Go module and build CoreDHCP:
   ```bash
   cd ../../../build
   go mod init coredhcp
   go mod edit -go=1.21
   go mod edit -replace=github.com/coredhcp/coredhcp=../coredhcp
   go mod edit -replace=github.com/OpenCHAMI/coresmd=../coresmd
   go mod tidy
   go build
   ```

Your `coredhcp` binary (including these two plugins) will be in the `./build` directory.

---

## Testing

Currently, the repository does not include standalone test scripts for these plugins. However, once compiled into CoreDHCP, you can test the overall DHCP server behavior using tools like:

- [dhcping](https://github.com/rickardw/dhcping)
- [dnsmasq DHCP client testing](http://www.thekelleys.org.uk/dnsmasq/doc.html)

Because the plugins integrate with SMD, also verify that your SMD instance is returning correct data, and confirm the environment variables (if using GoReleaser) are correctly embedded in the binary.

---

## Running

### Configuration

CoreDHCP requires a config file to run. See [`examples/coredhcp-config.yaml`](examples/coredhcp-config.yaml) for an example with detailed comments on how to enable and configure **coresmd** and **bootloop**.

### Preparation: SMD and BSS

Before running CoreDHCP, ensure the [OpenCHAMI](https://openchami.org) services (notably **BSS** and **SMD**) are configured and running. Their URLs should match what you configure in the CoreDHCP config file.

### Preparation: TFTP

By default, **coresmd** includes a built-in TFTP server with iPXE binaries for 32-/64-bit x86/ARM (EFI) and legacy x86. If you use the **bootloop** plugin and set the iPXE boot script path to `"default"`, it will serve a built-in reboot script to unknown nodes. Alternatively, you can point this to a custom TFTP path if different functionality is desired.

### Running CoreDHCP

Once all prerequisites are set, you can run CoreDHCP:

- **Docker**  
  Use host networking and mount your config file:
  ```bash
  docker run --rm \
    --net=host \
    -v /path/to/config.yaml:/etc/coredhcp/config.yaml:ro \
    ghcr.io/OpenCHAMI/coresmd:latest
  ```

- **Bare Metal**  
  Execute the locally built binary:
  ```bash
  ./coredhcp -conf /path/to/config.yaml
  ```

---

## More Reading

- [CoreDHCP GitHub](https://github.com/coredhcp/coredhcp)
- [OpenCHAMI Project](https://openchami.org)
- [SMD GitHub](https://github.com/OpenCHAMI/smd)
- [GoReleaser Documentation](https://goreleaser.com/install/)
- [Magellan (OpenCHAMI)](https://github.com/OpenCHAMI/magellan)

