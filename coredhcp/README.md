# CoreDHCP with CoreSMD Plugin

This directory contains the generated CoreDHCP server with the CoreSMD plugin integrated.

## Overview

The CoreSMD plugin extends CoreDHCP to provide dynamic DHCP services for OpenCHAMI cluster components by integrating with the State Management Database (SMD). It handles IP assignment, hostname generation, and iPXE boot configuration for both compute nodes and BMCs.

## Configuration

The CoreSMD plugin supports two configuration modes:

### New Format (Recommended)

Use a single YAML configuration file:

```yaml
server4:
  plugins:
    - server_id: 172.16.0.253
    - dns: 1.1.1.1 8.8.8.8
    - router: 172.16.0.254
    - netmask: 255.255.255.0
    - coresmd: /etc/coresmd/coresmd-config.yaml
```

The coresmd configuration file (`coresmd-config.yaml`) includes:
- SMD connection settings (URL, CA certificate, cache duration)
- Boot script configuration
- DHCP lease settings
- TFTP configuration
- **Hostname generation patterns for nodes and BMCs**

See `examples/coresmd-config.yaml` for a complete configuration example.

### Legacy Format (Backwards Compatible)

The plugin still supports the original 6-argument format:

```yaml
- coresmd: https://smd.url http://boot.url /path/to/ca.crt 30s 1h false
```

Arguments:
1. SMD base URL
2. Boot script base URL
3. CA certificate path (empty string for none)
4. Cache duration
5. Lease duration
6. Single port mode (true/false)

**Note:** The legacy format uses default hostname pattern `nid{04d}` for nodes only.

## Hostname Configuration

The new configuration file format allows customizable hostname patterns:

- **Node Pattern**: Generate hostnames for compute nodes
  - `nid{04d}` → `nid0001`, `nid0002`, ...
  - `dev-s{02d}` → `dev-s01`, `dev-s02`, ...
  - `{id}` → use xname directly (e.g., `x3000c0s0b0n0`)

- **BMC Pattern**: Generate hostnames for BMCs
  - `bmc{03d}` → `bmc001`, `bmc002`, ...
  - `{id}` → use xname directly (e.g., `x3000c0s0b1`)

- **Domain**: Optional domain suffix
  - `cluster.local` → `nid0001.cluster.local`
  - `dev-osc.lanl.gov` → `dev-s01.dev-osc.lanl.gov`

### Example Configurations

See the `examples/` directory for:
- `coresmd-config.yaml` - Full configuration with all options
- `coresmd-config-minimal.yaml` - Minimal working configuration
- `coresmd-config-lanl.yaml` - LANL deployment example

## Building

The `coredhcp.go` file is generated from the template and plugin list. To regenerate:

```bash
go generate ./generator
```

To build the server:

```bash
go build -o coredhcp ./coredhcp
```

## Running

```bash
./coredhcp -c /path/to/coredhcp-config.yaml
```
