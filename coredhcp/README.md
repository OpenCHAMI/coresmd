# CoreDHCP with CoreSMD Plugin

This directory contains the generated CoreDHCP server with the CoreSMD plugin integrated.

## Overview

The CoreSMD plugin extends CoreDHCP to provide dynamic DHCP services for OpenCHAMI cluster components by integrating with the State Management Database (SMD). It handles IP assignment, hostname generation, and iPXE boot configuration for both compute nodes and BMCs.

## Configuration

The CoreSMD plugin uses **key=value pairs** for clear, order-independent configuration. This format is recommended as it's more maintainable and less error-prone than positional arguments.

```yaml
server4:
  plugins:
    - server_id: 172.16.0.253
    - dns: 1.1.1.1 8.8.8.8
    - router: 172.16.0.254
    - netmask: 255.255.255.0
    - coresmd: smd_url=https://smd.url boot_script_url=http://boot.url cache_duration=30s lease_duration=1h
```

### Configuration Parameters

#### Required Parameters

- **`smd_url`** - SMD API base URL (must be `https://` or `http://`)
  - Example: `smd_url=https://smd.cluster.local`
  
- **`boot_script_url`** - Boot script URL for iPXE clients (HTTP, no TLS required)
  - Example: `boot_script_url=http://192.168.1.1:8081`
  
- **`cache_duration`** - Cache refresh interval for SMD data
  - Format: Go duration string (e.g., `30s`, `5m`, `1h`)
  - Example: `cache_duration=30s`
  
- **`lease_duration`** - DHCP lease time
  - Format: Go duration string (e.g., `1h`, `24h`, `7d`)
  - Example: `lease_duration=24h`

#### Optional Parameters

- **`ca_cert`** - Path to CA certificate file for SMD TLS validation
  - Omit this parameter or leave empty if SMD doesn't use TLS or uses publicly trusted certs
  - Example: `ca_cert=/etc/ssl/certs/my-ca.crt`
  
- **`single_port`** - Enable single port mode for NAT environments
  - Values: `true` or `false` (default: `false`)
  - Example: `single_port=true`
  
- **`node_pattern`** - Hostname pattern for compute nodes
  - Default: `nid{04d}` (generates `nid0001`, `nid0002`, etc.)
  - See [Hostname Patterns](#hostname-configuration) below
  - Example: `node_pattern="compute-{05d}"`
  
- **`bmc_pattern`** - Hostname pattern for BMCs
  - Default: empty (no hostnames assigned to BMCs)
  - See [Hostname Patterns](#hostname-configuration) below
  - Example: `bmc_pattern="bmc{03d}"`
  
- **`domain`** - Domain suffix to append to all hostnames
  - Default: empty (no domain suffix)
  - Example: `domain=cluster.local`

### Error Handling

The plugin validates all parameters on startup and provides clear error messages:
- **Missing required parameter**: `missing required parameter: smd_url`
- **Invalid format**: `invalid argument format 'noequals', expected key=value`
- **Invalid boolean**: `invalid single_port value 'maybe', use 'true' or 'false'`
- **Wrong argument count** (legacy format): `invalid arguments: use key=value format or legacy positional format (6 or 9 args), got 5 args`

### Backwards Compatibility

Legacy positional argument format is still supported for backwards compatibility:
- **6 arguments**: `smd_url boot_script_url ca_cert cache_duration lease_duration single_port`
- **9 arguments**: Same 6 arguments plus `node_pattern bmc_pattern domain`

However, the **key=value format is strongly recommended** as it's:
- More readable and self-documenting
- Order-independent (parameters can appear in any order)
- Easier to modify without breaking the configuration
- Less prone to copy-paste errors

## Hostname Configuration

The CoreSMD plugin generates DHCP hostnames for both compute nodes and BMCs using configurable patterns. This allows you to customize hostname formats to match your site's naming conventions.

### Pattern Syntax

Hostname patterns support two types of placeholders that get replaced with component-specific values:

#### `{Nd}` - Zero-Padded NID
Generates a zero-padded Node ID where `N` is the number of digits:
- **`{04d}`** → `0001`, `0002`, `0123`, `1234`
- **`{02d}`** → `01`, `02`, `23`, `99`
- **`{03d}`** → `001`, `002`, `042`, `999`
- **`{05d}`** → `00001`, `00042`, `12345`

The NID is extracted from the component's xname in SMD.

#### `{id}` - Component Xname
Uses the full component identifier from SMD:
- For compute nodes: `x3000c0s0b0n0`, `x3000c0s1b0n1`, etc.
- For BMCs: `x3000c0s0b1`, `x3000c0s1b1`, etc.

### Combining Patterns

You can mix literal text with placeholders to create custom naming schemes:

| Pattern | NID | Xname | Result |
|---------|-----|-------|--------|
| `nid{04d}` | 1 | x3000c0s0b0n0 | `nid0001` |
| `compute-{05d}` | 42 | x3000c0s1b0n1 | `compute-00042` |
| `dev-s{02d}` | 5 | x3000c0s0b0n0 | `dev-s05` |
| `node-{id}` | 123 | x3000c0s2b0n2 | `node-x3000c0s2b0n2` |
| `{id}` | 7 | x3000c0s0b1 | `x3000c0s0b1` |

### Domain Suffix

When the `domain` parameter is set, it's automatically appended to all generated hostnames:
- Pattern: `nid{04d}`, Domain: `cluster.local` → `nid0001.cluster.local`
- Pattern: `dev-s{02d}`, Domain: `hpc.example.com` → `dev-s01.hpc.example.com`

### Configuration Examples

#### Example 1: Default Configuration
Nodes get `nid####` hostnames, no BMC hostnames, no domain:
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=1h
```
**Results:**
- Node with NID 1: `nid0001`
- Node with NID 42: `nid0042`
- BMCs: No hostname assigned

#### Example 2: Custom Patterns with Domain
Five-digit node IDs, three-digit BMC IDs, with domain:
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h node_pattern="compute-{05d}" bmc_pattern="bmc{03d}" domain=hpc.local
```
**Results:**
- Node with NID 1: `compute-00001.hpc.local`
- Node with NID 42: `compute-00042.hpc.local`
- BMC with NID 1: `bmc001.hpc.local`
- BMC with NID 42: `bmc042.hpc.local`

#### Example 3: LANL-Style Naming
Short node IDs with site-specific domain:
```yaml
- coresmd: smd_url=https://smd.dev-osc.lanl.gov boot_script_url=http://172.16.0.253:8081 ca_cert=/etc/ssl/certs/lanl-ca.crt cache_duration=30s lease_duration=24h node_pattern="dev-s{02d}" bmc_pattern="bmc{03d}" domain=dev-osc.lanl.gov
```
**Results:**
- Node with NID 1: `dev-s01.dev-osc.lanl.gov`
- Node with NID 42: `dev-s42.dev-osc.lanl.gov`
- BMC with NID 1: `bmc001.dev-osc.lanl.gov`
- BMC with NID 42: `bmc042.dev-osc.lanl.gov`

#### Example 4: Using Xnames as Hostnames
Use full component identifiers for precise tracking:
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=1h node_pattern="{id}" bmc_pattern="{id}" domain=cluster.local
```
**Results:**
- Node: `x3000c0s0b0n0.cluster.local`
- Node: `x3000c0s1b0n1.cluster.local`
- BMC: `x3000c0s0b1.cluster.local`
- BMC: `x3000c0s1b1.cluster.local`

#### Example 5: Nodes Only, No BMC Hostnames
Assign hostnames to compute nodes but not BMCs:
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=1h node_pattern="worker{03d}" domain=k8s.local
```
**Results:**
- Node with NID 1: `worker001.k8s.local`
- Node with NID 42: `worker042.k8s.local`
- BMCs: No hostname assigned (bmc_pattern is empty)## Building

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
