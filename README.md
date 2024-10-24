# coresmd

<!-- Text width is 80, only use spaces and use 4 spaces instead of tabs -->
<!-- vim: set et sta tw=80 ts=4 sw=4 sts=0: -->

A CoreDHCP plugin with a pull-through cache that uses
[SMD](https://github.com/OpenCHAMI/smd) as its source of truth. This is part of
the [OpenCHAMI](https://openchami.org) project.

This repository contains two plugins:

- **coresmd** --- The purpose of this plugin is to provide DHCP leases based on
  data from SMD.
- **bootloop** --- The purpose of this plugin is to be a "catch-all" that causes
  nodes/BMCs/etc. unknown to SMD to get a temporary IP address from a pool with
  a short, user-defined lease time. This is so that if they are ever added to
  SMD, they will quickly get a longer lease from the **coresmd** plugin.

  An iPXE boot script that simply reboots is served to anything that can boot to
  force the whole DHCP handshake (DORA) to reoccur to obtain a new lease. If the
  plugin receives a DHCPREQUEST and its IP is already leased, a DHCPNAK is sent
  so that it will reinitiate the entire DHCP handshake.

  The goal is to have any MAC addresses that are unknown continually try and get
  a new IP address in the case they become known, but to also give them
  (especially BMCs) a temporary IP address so that they can be discovered (e.g.
  by [Magellan](https://github.com/OpenCHAMI/magellan).

## Building

This is meant to be built statically into
[CoreDHCP](https://github.com/coredhcp/coredhcp) using the
[coredhcp-generator](https://github.com/coredhcp/coredhcp/blob/master/cmds/coredhcp-generator).

### Container

This repository includes a Dockerfile that builds CoreDHCP with its core plugins
as well as this plugin.

```
docker build . --tag coresmd:latest
```

### Bare Metal

**NOTE:** Certain source files in CoreDHCP only build on Linux, which will cause
build errors when building on other platforms like Mac.

It is recommended to do this within a clean directory.

1. Create directory for generated source files:

   ```
   mkdir coresmd
   ```

1. Clone CoreDHCP and change the working directory to the coredhcp-generator
   tool.

   ```
   git clone https://github.com/coredhcp/coredhcp
   cd coredhcp/cmds/coredhcp-generator
   ```

1. Build the generator.

   ```
   go mod download
   go build
   ```

1. Run the generator to generate the CoreDHCP source file.

   ```
   ./coredhcp-generator -f core-plugins.txt -t coredhcp.go.template -o ../../../coresmd/coredhcp.go \
     github.com/OpenCHAMI/coresmd/coresmd \
     github.com/OpenCHAMI/coresmd/bootloop
   ```

   You should see output similar to the following:

   ```
   2024/10/23 22:57:51 Generating output file '../../../coresmd/coredhcp.go' with 17 plugin(s):
   2024/10/23 22:57:51   1) github.com/OpenCHAMI/coresmd/bootloop
   2024/10/23 22:57:51   2) github.com/coredhcp/coredhcp/plugins/autoconfigure
   2024/10/23 22:57:51   3) github.com/coredhcp/coredhcp/plugins/file
   2024/10/23 22:57:51   4) github.com/coredhcp/coredhcp/plugins/prefix
   2024/10/23 22:57:51   5) github.com/coredhcp/coredhcp/plugins/leasetime
   2024/10/23 22:57:51   6) github.com/coredhcp/coredhcp/plugins/mtu
   2024/10/23 22:57:51   7) github.com/coredhcp/coredhcp/plugins/nbp
   2024/10/23 22:57:51   8) github.com/coredhcp/coredhcp/plugins/router
   2024/10/23 22:57:51   9) github.com/coredhcp/coredhcp/plugins/searchdomains
   2024/10/23 22:57:51  10) github.com/coredhcp/coredhcp/plugins/dns
   2024/10/23 22:57:51  11) github.com/coredhcp/coredhcp/plugins/netmask
   2024/10/23 22:57:51  12) github.com/coredhcp/coredhcp/plugins/range
   2024/10/23 22:57:51  13) github.com/coredhcp/coredhcp/plugins/serverid
   2024/10/23 22:57:51  14) github.com/coredhcp/coredhcp/plugins/sleep
   2024/10/23 22:57:51  15) github.com/coredhcp/coredhcp/plugins/staticroute
   2024/10/23 22:57:51  16) github.com/OpenCHAMI/coresmd/coresmd
   2024/10/23 22:57:51  17) github.com/coredhcp/coredhcp/plugins/ipv6only
   2024/10/23 22:57:51 Generated file '../../../coresmd/coredhcp.go'. You can build it by running 'go build' in the output directory.
   ../../../coresmd
   ```

1. Change directory into the directory, initialize it as a Go module.

   ```
   cd ../../../coresmd
   go mod init coredhcp   # the module name doesn't matter
   go mod edit -go=1.21
   go mod edit -replace=github.com/coredhcp/coredhcp=../coredhcp
   ```
   **Only if you have coresmd checked out locally**
   ```
   go mod edit -replace=github.com/OpenCHAMI/coresmd=<path_to_checkout>
   ```
   Finally:
   ```
   go mod tidy
   ```

1. Build CoreDHCP.

   ```
   go build
   ```

You'll now have a `coredhcp` binary in the current directory you can run.

## Configuration

CoreDHCP requires a config file to run. An example `config.yaml` can be found at
`dist/config.example.yaml`. That file contains comments on when/how to use the
coresmd and bootloop plugins, including which arguments to pass.

## Usage

### Preparation: SMD and BSS

Before running CoreDHCP, the OpenCHAMI services (namely BSS and SMD) should
already be configured and running using the base URL and boot script base URL
configured in the CoreDHCP config file.

### Preparation: TFTP

Neither CoreDHCP nor this plugin provide TFTP capability, so a separate TFTP
server is required to be running[^tftp]. The IP address that this server listens
on should match the `server_id` directive in the CoreDHCP config file. This
server should serve the following files:

- `reboot.ipxe` --- This file is located `dist/` in this repository.
- `ipxe.efi` --- The iPXE x86\_64 EFI bootloader. This can be found
  [here](https://boot.ipxe.org/ipxe.efi).
- `undionly.kpxe` --- The iPXE x86 legacy bootloader. This can be found
  [here](https://boot.ipxe.org/undionly.kpxe).

[^tftp]: [Here](https://github.com/aguslr/docker-atftpd) is one that is easy to
    get running.

### Running CoreDHCP

After the above prerequisites have been completed, CoreDHCP can be run with its
config file. It can be run in a container or on bare metal, though if running
via container host networking is required.

For example, to run using Docker:

```
docker run --rm -v <path_to_config_file>:/etc/coredhcp/config.yaml:ro ghcr.io/OpenCHAMI/coresmd:latest
```
