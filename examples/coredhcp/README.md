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
  - [DHCPv4 Zero-Touch Provisioning](#dhcpv4-zero-touch-provisioning)

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
  - coresmd: svc_base_uri=https://foobar.openchami.cluster ipxe_uri=http://172.16.0.253:8081/bootscript ca_cert=/root_ca/root_ca.crt cache_valid=30s lease_time=1h single_port=false
```

So as to not have an endless text line, a YAML multi-line string can also be used to separate the arguments:

```yaml
plugins:
  - coresmd: |
      svc_base_uri=https://foobar.openchami.cluster
      ipxe_uri=http://172.16.0.253:8081/bootscript
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
      /* Boot script URI */
      ipxe_uri=http://172.16.0.253:8081/bootscript
      /* CA trust bundle */
      ca_cert=/root_ca/root_ca.crt
      /* SMD cache validity duration */
      cache_valid=30s
      /* lease duration */
      lease_time=1h single_port=false
```

See [coredhcp.yaml](coredhcp.yaml) for a full example with documentation comments.

## DHCPv4 and DHCPv6 Support

The **coresmd** plugin supports both DHCPv4 (via `server4`) and DHCPv6 (via `server6`) configurations. Both protocols use the same configuration format and share these features:

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
      ipxe_uri=http://[fd00:100::254]:8081/bootscript
      ca_cert=/root_ca/root_ca.crt
      cache_valid=30s
      lease_time=1h
      node_pattern=nid{04d}
      bmc_pattern=bmc{04d}
      domain=openchami.cluster
```

See [coredhcp.yaml](coredhcp.yaml) for a complete example showing both DHCPv4 and DHCPv6 configurations.

## Custom Hostnames

Hostname patterns can be used to specify custom hostnames for nodes and BMCs. See [**hostnames.md**](hostnames.md) for more details.

## DHCPv4 Zero-Touch Provisioning

Use an optional `ztp_user_class=CLASS,URL` mapping to give known provisioning
clients a complete configuration URL in DHCP option 67 instead of an iPXE
bootloader or boot script:

```yaml
server4:
  plugins:
    - server_id: 192.0.2.1
    - coresmd: |
        svc_base_uri=https://smd.example.test
        ipxe_uri=https://boot.example.test/bootscript
        ca_cert=/etc/openchami/root_ca.crt
        lease_time=1h
        ztp_user_class=NVOS-ZTP,https://ztp.example.test/ztp.json
```

The class matches a complete, case-sensitive DHCP option 77 user class. Both
RFC 3004 lists of length-prefixed classes and legacy unprefixed strings are
accepted. This allows an NVOS client identifying as `NVOS-ZTP` to receive its
ZTP URL while other clients retain their usual iPXE behavior. There is no
universal DHCP flag identifying every ZTP-capable device.

Repeat the mapping for other client classes. If several classes match, the
first mapping in configuration order wins. Duplicate configured classes are
rejected. The class must be 1-255 printable ASCII bytes without whitespace or
commas; the URL must be absolute and include a host. Commas and equals signs
inside the URL are preserved. The configured URL is sent unchanged, without
the MAC query parameter added to iPXE boot scripts.

CoreSMD still requires the client's MAC and IP address in SMD. Normal lease,
hostname, router, netmask and ignore rules run before boot selection. Both
OFFER and ACK responses include the selected URL. A matching ZTP response
removes any earlier option 66 to prevent clients from combining a TFTP server
name with the complete URL. Unrecognized MAC addresses still pass to later
plugins; this mapping does not allocate fallback addresses.

Option 77 is supplied by the client and is a selector, not authentication.
Configure access to provisioning content separately. This feature applies to
DHCPv4 only; DHCPv6 boot selection is unchanged.

This SMD-backed path does not require a separate `nbp` plugin. The CoreDHCP
`nbp ... continue` enhancement addresses composition with other allocators
such as `file` and `range`.
