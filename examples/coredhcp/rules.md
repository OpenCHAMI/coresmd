<!--
SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Rich Rules for the CoreSMD CoreDHCP Plugin

Rich rules allow specifying various DHCP options for certain machines using
match filters.

## Contents

- [Rich Rules for the CoreSMD CoreDHCP Plugin](#rich-rules-for-the-coresmd-coredhcp-plugin)
  - [Contents](#contents)
  - [Quick Start](#quick-start)
  - [Summary](#summary)
  - [CoreSMD Configuration Directives](#coresmd-configuration-directives)
    - [`domain=DOMAIN`](#domaindomain)
    - [`rule_log={info|debug|none}`](#rule_loginfodebugnone)
    - [`rule=RULE`](#rulerule)
  - [Migrating from `*_pattern`](#migrating-from-_pattern)
  - [Pattern Syntax](#pattern-syntax)
    - [`{Nd}` - Zero-Padded NID](#nd---zero-padded-nid)
    - [`{id}` - Component Xname](#id---component-xname)
  - [Rule Syntax](#rule-syntax)
    - [Match Keys](#match-keys)
      - [`type:TYPE[|TYPE...]`](#typetypetype)
      - [`subnet:CIDR[|CIDR...]`](#subnetcidrcidr)
      - [`id:XNAME`](#idxname)
      - [`id_set:EXPR`](#id_setexpr)
    - [Action Keys](#action-keys)
      - [`hostname:PATTERN`](#hostnamepattern)
      - [`routers:IP[|IP...]`](#routersipip)
      - [`netmask:MASK`](#netmaskmask)
      - [`cidr:BITS`](#cidrbits)
      - [`domain:DOMAIN`](#domaindomain-1)
      - [`domain_append:MODE`](#domain_appendmode)
      - [`continue:{true|false}`](#continuetruefalse)
      - [`name:STRING`](#namestring)
      - [`log:{info|debug|none}`](#loginfodebugnone)
  - [Rule Ordering](#rule-ordering)
  - [Examples](#examples)
    - [1. Minimal: NID hostnames for nodes only](#1-minimal-nid-hostnames-for-nodes-only)
    - [2. Add a domain suffix](#2-add-a-domain-suffix)
    - [3. Per-type rules](#3-per-type-rules)
    - [4. Subnet-specific BMC naming](#4-subnet-specific-bmc-naming)
    - [5. Per-subnet routers (DHCPv4 option 3)](#5-per-subnet-routers-dhcpv4-option-3)
    - [6. Netmask and CIDR (DHCPv4 option 1)](#6-netmask-and-cidr-dhcpv4-option-1)
    - [7. Suppress domain suffix for selected hosts](#7-suppress-domain-suffix-for-selected-hosts)
    - [8. Hostname override with `continue`](#8-hostname-override-with-continue)
    - [9. Familiar "legacy-style" configuration, expressed as rules](#9-familiar-legacy-style-configuration-expressed-as-rules)
  - [Caveats](#caveats)
  - [See Also](#see-also)

## Quick Start

In a CoreDHCP configuration:

```yaml
- coresmd: |
    svc_base_uri=https://smd.cluster.local
    ipxe_base_uri=http://192.168.1.1
    cache_valid=30s
    lease_time=24h
    domain=cluster.local
    rule_log=info

    /* Set node hostnames */
    rule=type:Node,hostname:nid{04d}

    /* Set BMC hostnames */
    rule=type:NodeBMC,hostname:{id}
```

## Summary

CoreSMD can set various DHCP options (e.g. hostname (option 12), routers (option
3), netmask (option 1), etc.) using an ordered list of rules. Each rule matches on SMD inventory
attributes (component type, component ID, and the assigned IP address) and sets
the appropriate DHCP options based on the actions defined in the rule.

Rules are evaluated in order. A matching rule sets the relevant DHCP options
and may halt evaluation (the default or if `continue:false`) or allow
evaluation to continue (`continue:true`).

Within a `coresmd` configuration block, comments use C-style delimiters: `/* ...
*/`.

## CoreSMD Configuration Directives

### `domain=DOMAIN`

The global domain suffix. When set, it is appended to hostnames unless a rule
specifies its own domain behavior. Leading dots are ignored.

**Default:** none

### `rule_log={info|debug|none}`

Controls rule logging.

- `info`: log matching rules (rule name, match keys, resulting hostname)
- `debug`: log matching and non-matching rules (useful for troubleshooting)
- `none`: disable rule logging

**Default:** `info`

### `rule=RULE`

Add a rule. May be specified multiple times. **Order matters!**

A rule may set:

- `hostname` (DHCP option 12)
- `routers` (DHCPv4 option 3)
- `netmask` (DHCPv4 option 1)

## Migrating from `*_pattern`

Older CoreSMD configurations used legacy pattern directives (for example
`node_pattern` and `bmc_pattern`). These are replaced by `rule=`.

The equivalent `rule=` forms are:

| **Old Config** | **New Config** |
|---|---|
| `node_pattern=PATTERN` | `rule=type:Node,hostname:PATTERN`    |
| `bmc_pattern=PATTERN`  | `rule=type:NodeBMC,hostname:PATTERN` |

When migrating, place these rules where you want them in the evaluation order.
It's common to put more specific rules before general ones so that the general
ones act as a catch-all.

## Pattern Syntax

### `{Nd}` - Zero-Padded NID

Generate a zero-padded Node ID where `N` is the number of digits:

| Pattern | NID | Result |
|---------|-----|--------|
| `{02d}` | 1   | `01`   |
| `{02d}` | 42  | `42`   |
| `{03d}` | 1   | `001`  |
| `{03d}` | 42  | `042`  |
| `{04d}` | 1   | `0001` |
| `{04d}` | 42  | `0042` |
| `{05d}` | 1   | `00001`|
| `{05d}` | 123 | `00123`|

### `{id}` - Component Xname

Use the full component identifier from SMD:

- Nodes: `x3000c0s0b0n0`, `x3000c0s1b0n1`, ...
- BMCs: `x3000c0s0b1`, `x3000c0s1b1`, ...

## Rule Syntax

`RULE` is a comma-separated list of `key:val` components organized as match
key-value pairs followed by action key-value pairs:

```
rule=<match_keyval(s)>,<action_keyval(s)>
```

A basic example of this is:

```
rule=type:Node,subnet:172.16.0.0/21,hostname:nid{04d}
```

(`type` and `subnet` are match key-value pairs, `hostname` is an action.)

Values may be quoted with single or double quotes:

```
rule=type:Node,hostname:"rack42-{id}"
```

Unknown keys are rejected as an error. Duplicate keys in a rule are rejected as
an error.

If no rule matches, CoreSMD falls back to the built-in default hostname pattern:
`unknown-{04d}`

### Match Keys

If a match key is omitted, it does not constrain the match. For example, if
`type` is omitted, any type (conforming to any further constraints) will match.

#### `type:TYPE[|TYPE...]`

Match component type as reported by SMD, e.g. `Node`, `NodeBMC`, `HSNSwitch`.
Tokens are whitespace-trimmed. If `type:` is present it must contain at least
one type.

**Default:** omitted (matches all types)

#### `subnet:CIDR[|CIDR...]`

Match assigned IP against one or more CIDRs. If a component in SMD has more
than one IP address assigned, **only the first IP in the list is matched
against `subnet`** because CoreDHCP only assigns one address to the interface
and it is assumed that the first IP address in the list is the desired one.

`subnet` is the only match key that can also serve as an action key. See
[`netmask:MASK`](#netmaskmask) and [`cidr:BITS`](#cidrbits).

**Default:** omitted (matches any subnet)

#### `id:XNAME`

Match the component's ID (xname).

Mutually exclusive with `id_set`.

**Default:** omitted (matches any ID)

#### `id_set:EXPR`

Match a set of component IDs.

**NOTE:** `id_set` is reserved; rules using it will fail to parse until the
ID-set matcher is implemented.

Mutually exclusive with `id`.

### Action Keys

Rules may apply one or more actions when matched. At least one action must be
specified (`hostname`, `routers`, `netmask`, or `cidr`). If no match keys are
specified, the action(s) will apply to all incoming DHCP requests.

#### `hostname:PATTERN`

Hostname to apply. This value supports the same pattern specifiers as the
legacy `pattern` key. See [Pattern Syntax](#pattern-syntax) above.

**Default:** omitted (no hostname set by this rule)

#### `routers:IP[|IP...]`

Set DHCPv4 Router option (RFC 2132 option 3) for the matched host. Values must
be IPv4 addresses. Multiple routers may be specified using `|`. This action
will override any routers set with the CoreDHCP `netmask` plugin.

This action applies to DHCPv4 only.

**Default:** omitted (no routers set by this rule)

#### `netmask:MASK`

Set DHCPv4 Subnet Mask option (RFC 2132 option 1) for the matched host. The
mask must be a valid IPv4 network mask (contiguous 1-bits), for example:

- `255.255.255.0`
- `255.255.252.0`

This action applies to DHCPv4 only.

Mutually exclusive with `cidr`.

If a rule uses the `subnet` match key and neither `netmask` nor `cidr` is
specified, CoreSMD sets the DHCPv4 Subnet Mask option to the mask of the **first
subnet** in the rule.

**Default:** omitted

#### `cidr:BITS`

Set DHCPv4 Subnet Mask option (RFC 2132 option 1) for the matched host by CIDR
width, where `BITS` is an integer from 1 to 32 (inclusive), for example:

- `24` (equivalent to `255.255.255.0`)
- `21` (equivalent to `255.255.248.0`)

This action applies to DHCPv4 only.

Mutually exclusive with `netmask`.

If a rule uses the `subnet` match key and neither `netmask` nor `cidr` is
specified, CoreSMD sets the DHCPv4 Subnet Mask option to the mask of the **first
subnet** in the rule.

**Default:** omitted

#### `domain:DOMAIN`

Domain to use with hostname if rule matches. Leading dots are ignored. See
`domain_append` below for modification of how domain behavior is applied.

**Default:** omitted (use global `domain` if set, otherwise none)

#### `domain_append:MODE`

Controls how domains are appended to the hostname.

Each MODE and its effect are detailed below:

| **`domain_append`** | **Final Hostname** |
|---|---|
| `global` | `<hostname>.<global_domain>` |
| `rule` | `<hostname>.<rule_domain>` |
| `global\|rule` | `<hostname>.<global_domain>.<rule_domain>` |
| `rule\|global` | `<hostname>.<rule_domain>.<global_domain>` |
| `none` | `<hostname>` |

Order matters for the combined forms (`global|rule` vs `rule|global`).

If a domain requested by `domain_append` is not set (for example `global` when
the global `domain` is unset), that portion is omitted.

**Default when omitted:**

| **Global `domain` Set?** | **Rule `domain` Set?** | **Result** |
|---|---|---|
| yes | no | append global domain |
| yes | yes | append rule domain (overrides global) |
| no | no | no domain appended |
| no | yes | append rule domain |

#### `continue:{true|false}`

Controls whether to continue evaluating subsequent rules if the rule matches.

If `true`, evaluation continues after a match.

If `false`, evaluation ceases after a match and the hostname is final.

**Default:** `false`

#### `name:STRING`

Rule identifier used in logs.

**Default:** a generated name based on hash of rule (e.g. rule-b176a842)

#### `log:{info|debug|none}`

Per-rule logging override.

Possible values:

- `info`: log only matches for this rule
- `debug`: log matches and non-matches for this rule
- `none`: suppress rule logging for this rule

**Default:** value of `rule_log`

## Rule Ordering

Rules are evaluated in the order they appear.

- If a rule matches and `continue:false`, evaluation stops.
- If a rule matches and `continue:true`, evaluation continues.
- If multiple rules match, the hostname reflects the last matching rule that
  stops evaluation (or the last rule in the list).
- If no rule matches, CoreSMD falls back to compile-time default of
  `unknown-{04d}`.

When `continue:true`, the computed hostname _from the last matching rule_ is
kept and may be modified/overridden by later matching rules. This is useful for
generic naming followed by narrow overrides.

## Examples

### 1. Minimal: NID hostnames for nodes only

```yaml
- coresmd: |
    svc_base_uri=https://smd.cluster.local
    ipxe_base_uri=http://192.168.1.1
    rule=type:Node,hostname:nid{04d}
```

Assign node hostnames from NID. Other types fall back to the default hostname
(`unknown-{04d}`) unless additional rules are added.

| NID | Type | Hostname |
|-----|------|----------|
| 1   | Node | `nid0001` |
| 42  | Node | `nid0042` |

### 2. Add a domain suffix

```yaml
- coresmd: |
    domain=cluster.local
    rule=type:Node,hostname:nid{04d}
    rule=type:NodeBMC,hostname:bmc{04d}
```

Assign hostnames for nodes and BMCs and append a global domain.

| NID | Type | Hostname |
|-----|------|----------|
| 1   | Node    | `nid0001.cluster.local` |
| 1   | NodeBMC | `bmc0001.cluster.local` |

### 3. Per-type rules

```yaml
- coresmd: |
    domain=lab.local
    rule=type:Node,hostname:compute-{05d}
    rule=type:NodeBMC,hostname:ipmi-{05d}
    rule=type:HSNSwitch,hostname:{id}
```

Use separate naming schemes per component type.

| Input | Type | Hostname |
|-------|------|----------|
| NID=7 | Node | `compute-00007.lab.local` |
| NID=7 | NodeBMC | `ipmi-00007.lab.local` |
| ID=`x3000c0h0s0` | HSNSwitch | `x3000c0h0s0.lab.local` |

### 4. Subnet-specific BMC naming

```yaml
- coresmd: |
    domain=lab.local
    rule=type:NodeBMC,subnet:172.16.10.0/24,hostname:bmc10-{04d}
    rule=type:NodeBMC,subnet:172.16.11.0/24,hostname:bmc11-{04d}
    rule=type:NodeBMC,hostname:bmc-{04d}
```

Name BMCs differently depending on the assigned subnet, with a final fallback
rule for BMCs.

| IP (assigned) | Type | NID | Hostname |
|--------------:|------|-----|----------|
| 172.16.10.99  | NodeBMC | 7 | `bmc10-0007.lab.local` |
| 172.16.11.50  | NodeBMC | 7 | `bmc11-0007.lab.local` |
| 172.16.12.10  | NodeBMC | 7 | `bmc-0007.lab.local` |

### 5. Per-subnet routers (DHCPv4 option 3)

Use the `routers:` action to set per-node default gateways. This applies to DHCPv4 only.

```yaml
- coresmd: |
    domain=lab.local

    /* Default router for most nodes */
    rule=type:Node,hostname:nid{04d},routers:172.16.0.1

    /* Management subnet uses a different router */
    rule=type:Node,subnet:172.16.0.0/24,hostname:nid{04d},routers:172.16.0.254

    /* Multiple routers may be specified (in order) */
    rule=type:Node,subnet:172.16.1.0/24,hostname:nid{04d},routers:172.16.1.1|172.16.1.2
```

The hostname is still assigned from the hostname pattern; the `routers` list is written
to DHCP option 3 for matching DHCPv4 clients.

| IP (assigned) | Type | NID | Hostname | Routers (option 3) |
|--------------:|------|-----|----------|--------------------|
| 172.16.0.10 | Node | 7 | `nid0007.lab.local` | `172.16.0.254` |
| 172.16.1.10 | Node | 7 | `nid0007.lab.local` | `172.16.1.1`, `172.16.1.2` |
| 172.16.9.10 | Node | 7 | `nid0007.lab.local` | `172.16.0.1` |

### 6. Netmask and CIDR (DHCPv4 option 1)

Use `netmask:` or `cidr:` to set the DHCPv4 subnet mask (option 1).

If a rule uses `subnet:` as a match key and neither `netmask` nor `cidr` is
specified, CoreSMD sets option 1 to the mask of the **first subnet** in the
rule.

```yaml
- coresmd: |
    domain=lab.local

    /* Explicit mask */
    rule=name:mask,type:Node,hostname:nid{04d},netmask:255.255.255.0

    /* Explicit CIDR */
    rule=name:cidr,type:NodeBMC,hostname:bmc{04d},cidr:26

    /* Implicit mask from subnet (first subnet only) */
    rule=name:implicit,type:Node,subnet:172.16.0.0/21|172.16.8.0/21,hostname:nid{04d}
```

| Rule | Type | IP (assigned) | Hostname | Netmask (option 1) |
|---|---|---:|---|---|
| `mask` | Node | 172.16.0.10 | `nid0007.lab.local` | `255.255.255.0` |
| `cidr` | NodeBMC | 172.16.0.10 | `bmc0007.lab.local` | `255.255.255.192` |
| `implicit` | Node | 172.16.0.10 | `nid0007.lab.local` | `255.255.248.0` |

### 7. Suppress domain suffix for selected hosts

Use `domain_append:none` in the rule action. This suppresses all domain
suffixing, regardless of global `domain` and regardless of other
`domain_append` settings.

```yaml
- coresmd: |
    domain=.cluster.local
    rule=type:Node,hostname:nid{04d}

    /* BMCs get *no* domain suffix */
    rule=type:NodeBMC,hostname:bmc{04d},domain_append:none
```

Use `domain_append:none` to suppress suffixing.

| NID | Type | Hostname |
|-----|------|----------|
| 1 | Node | `nid0001.cluster.local` |
| 1 | NodeBMC | `bmc0001` |

### 8. Hostname override with `continue`

`continue:true` allows later rules to refine/override hostnames.

```yaml
- coresmd: |
    domain=cluster.local

    /* Base node naming */
    rule=name:base,type:Node,hostname:nid{04d},continue:true

    /* Override domain for a subset of nodes (example) */
    rule=name:mgmt,type:Node,subnet:172.16.0.0/24,hostname:nid{04d},domain:mgmt.local
```

Two rules match nodes in `172.16.0.0/24`. The first sets the baseline hostname;
the second overrides the domain for that subnet.

| IP (assigned) | Type | NID | Hostname |
|--------------:|------|-----|----------|
| 172.16.0.10 | Node | 7 | `nid0007.mgmt.local` |
| 172.16.9.10 | Node | 7 | `nid0007.cluster.local` |

### 9. Familiar "legacy-style" configuration, expressed as rules

The old `node_pattern`, `bmc_pattern`, `hostname_by_type`, and
`hostname_default` directives are replaced by `rule`.

```yaml
- coresmd: |
    domain=cluster.local

    /* Equivalent to: node_pattern=nid{04d} */
    rule=type:Node,hostname:nid{04d}

    /* Equivalent to: bmc_pattern=bmc{04d} */
    rule=type:NodeBMC,hostname:bmc{04d}

    /* Equivalent to: hostname_by_type=HSNSwitch:{id} */
    rule=type:HSNSwitch,hostname:{id}

    /* Equivalent to: hostname_default=... (final fallback) */
    rule=hostname:unknown-{04d}
```

This produces the same administrator-facing naming scheme as the legacy knobs,
while preserving rule ordering and allowing additional matching criteria.

## Multiple Subnet Support

CoreSMD provides two complementary mechanisms for multi-subnet environments:

### Config-Level `subnet=`

The `subnet=CIDR,ROUTER` directive configures subnet-aware IP selection and
provides baseline DHCP options (router and netmask) based on the relay agent's
giaddr:

```yaml
- coresmd: |
    subnet=10.40.1.0/24,10.40.1.1
    subnet=10.40.3.0/24,10.40.3.1
```

When configured, CoreSMD uses giaddr to determine which subnet the DHCP request
came from, filters the component's IPs to those belonging to that subnet, and
sets the corresponding router (option 3) and netmask (option 1).

### Rule-Level `subnet:`

Rules can also match on subnets and set per-rule routers and netmask:

```yaml
- coresmd: |
    rule=subnet:10.40.1.0/24,hostname:compute-{04d},routers:10.40.1.1,cidr:24
    rule=subnet:10.40.3.0/24,hostname:storage-{04d},routers:10.40.3.1,cidr:24
```

### Order of Operations

When both are configured:

1. **Config-level `subnet=`** sets baseline router and netmask options
2. **`rule=...` evaluation** runs after, and matching rules **override** the
   baseline options via `resp.Options.Update()`

This means rule-level actions always take precedence over config-level subnet
settings. If you only need subnet-aware IP selection (giaddr filtering) without
per-subnet hostnames, use `subnet=` alone. If you need per-subnet hostnames,
routers, or netmasks, use rules with `subnet:` match keys.

## Caveats

- At least one action must be specified: `hostname`, `routers`, `netmask`, or `cidr`.
  (Note: `subnet` may implicitly set a netmask when neither `netmask` nor `cidr` is specified.)
- `type:` is optional, but if present must include at least one type.
- Only the first assigned IP is used for subnet matching.
- `domain_append:none` suppresses all domain suffixing.
- `id_set` is reserved; rules using it will fail to parse until implemented.

If you do not want the built-in fallback (`unknown-{04d}`) to apply, provide a
final "catch-all" rule that matches your desired behavior, e.g.:

```yaml
- coresmd: |
    ...
    rule=hostname:{id}
```

## See Also

CoreDHCP configuration documentation and SMD inventory documentation.
