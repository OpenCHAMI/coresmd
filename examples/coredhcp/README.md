<!--
SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# CoreDHCP Configuration Examples for CoreSMD

This directory contains example CoreDHCP configurations for the CoreSMD plugin.

## Contents

- [CoreDHCP Configuration Examples for CoreSMD](#coredhcp-configuration-examples-for-coresmd)
  - [Contents](#contents)
  - [DHCPv4 and DHCPv6 Support](#dhcpv4-and-dhcpv6-support)
  - [Positional vs. Key-Value Format](#positional-vs-key-value-format)
  - [Custom Hostnames](#custom-hostnames)

## Positional vs. Key-Value Format

Prior to CoreSMD v0.5.0, positional arguments were used to configure CoreSMD which made it difficult to match configuration values to configuration keys. An example of such configuration would be:

```yaml
plugins:
  - coresmd: https://foobar.openchami.cluster http://172.16.0.253:8081 /root_ca/root_ca.crt 30s 1h false
```

With fresh eyes, it's difficult to see what these values mean. With CoreSMD v0.5.0 and beyond, key-value pairs are used instead. The format is `key=value` with no spaces on either side of the equal sign (think [Linux kernel command line](https://www.man7.org/linux/man-pages/man7/bootparam.7.html)).

To migrate the above configuration to the new format, it would become:

```yaml
plugins:
  - coresmd: svc_base_uri=https://foobar.openchami.cluster ipxe_base_uri=http://172.16.0.253:8081 ca_cert=/root_ca/root_ca.crt cache_valid=30s lease_time=1h single_port=false
```

So as to not have an endless text line, a YAML multi-line string can also be used to separate the arguments:

```yaml
plugins:
  - coresmd: |
      svc_base_uri=https://foobar.openchami.cluster
      ipxe_base_uri=http://172.16.0.253:8081
      ca_cert=/root_ca/root_ca.crt
      cache_valid=30s
      lease_time=1h single_port=false
```

Comments are supported too:

```yaml
plugins:
  - coresmd: |
      /* SMD base URI */
      svc_base_uri=https://foobar.openchami.cluster
      /* BSS boot script base URI */
      ipxe_base_uri=http://172.16.0.253:8081
      /* CA trust bundle */
      ca_cert=/root_ca/root_ca.crt
      /* SMD cache validity duration */
      cache_valid=30s
      /* lease duration */
      lease_time=1h single_port=false
```

See [coredhcp.yaml](coredhcp.yaml) for a full example with documentation comments.

## DHCPv4 and DHCPv6 Support

The **coresmd** plugin supports both DHCPv4 (via `server4`) and DHCPv6 (via `server6`) configurations. Both protocols use the same configuration format and support the same features:

- IP address assignment from SMD inventory
- Custom hostname patterns for nodes and BMCs
- TFTP boot configuration for iPXE
- Configurable lease times and cache validity

### DHCPv6 Considerations

- **IPv6 Address Assignment**: The plugin will automatically select IPv6 addresses from the `IPAddresses` field in SMD's EthernetInterfaces. If both IPv4 and IPv6 addresses are present, DHCPv4 will use IPv4 addresses and DHCPv6 will use IPv6 addresses.
- **FQDN Support**: DHCPv6 uses the FQDN option to set hostnames, following RFC 4704.
- **Boot Configuration**: DHCPv6 supports boot file URL options for network booting with iPXE.
- **Lease Times**: DHCPv6 uses IANA (Identity Association for Non-temporary Addresses) with T1 and T2 timers calculated from the configured lease time.

### Example DHCPv6 Configuration

```yaml
server6:
  listen:
    - "[::]:547"  # Listen on all IPv6 interfaces

plugins:
  - server_id: LL 00:de:ad:be:ef:00
  - dns: fd00:100::254
  - coresmd: |
      svc_base_uri=https://smd.openchami.cluster
      ipxe_base_uri=http://[fd00:100::254]:8081
      ca_cert=/root_ca/root_ca.crt
      cache_valid=30s
      lease_time=1h
      node_pattern=nid{04d}
      bmc_pattern=bmc{04d}
      domain=openchami.cluster
```

See [coredhcp.yaml](coredhcp.yaml) for a complete example showing both DHCPv4 and DHCPv6 configurations.

## TokenSmith Auth for SMD Requests

The CoreDHCP `coresmd` plugin can authenticate outbound SMD API requests using
TokenSmith service tokens.

Configure auth in the `coresmd` key-value block:

- `auth_mode={disabled|optional|required}`
  - `disabled` (default): no auth
  - `optional`: try auth, continue unauthenticated if bootstrap exchange fails
  - `required`: fail startup if bootstrap exchange fails
- `tokensmith_url=https://tokensmith.cluster.local`
  - required for `optional` and `required`
- `refresh_before=2m`
  - optional lead time before token expiry for proactive refresh

Set the bootstrap token in the environment for the CoreDHCP process:

```bash
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-token>"
```

Example:

```yaml
plugins:
  - coresmd: |
      svc_base_uri=https://smd.openchami.cluster
      ipxe_base_uri=http://172.16.0.253:8081
      ca_cert=/root_ca/root_ca.crt
      cache_valid=30s

      auth_mode=required
      tokensmith_url=https://tokensmith.cluster.local
      refresh_before=90s

      rule=type:Node,hostname:nid{04d}
      rule=type:NodeBMC,hostname:bmc{04d}
      domain=openchami.cluster
```

`target_service` and `scopes` are intentionally omitted from plugin config.
TokenSmith derives both from bootstrap token claims.

## Custom Hostnames

Hostname patterns can be used to specify custom hostnames for nodes and BMCs. See [**hostnames.md**](hostnames.md) for more details.
