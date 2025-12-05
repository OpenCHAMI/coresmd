# CoreSMD Hostname Pattern Examples

This document provides detailed examples of hostname pattern configuration for the CoreSMD CoreDHCP plugin.

## Overview

The CoreSMD plugin generates DHCP hostnames for compute nodes and BMCs using configurable patterns. This allows you to customize hostname formats to match your site's naming conventions.

## Pattern Syntax

### `{Nd}` - Zero-Padded NID

Generates a zero-padded Node ID where `N` is the number of digits:

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

Uses the full component identifier from SMD:
- Compute nodes: `x3000c0s0b0n0`, `x3000c0s1b0n1`, etc.
- BMCs: `x3000c0s0b1`, `x3000c0s1b1`, etc.

## Complete Examples

### Example 1: Default Configuration (HPC Standard)

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h
```

**Result:**
- Nodes: `nid0001`, `nid0002`, `nid0123`, `nid1234`
- BMCs: No hostname assigned

**Use case:** Standard HPC cluster with traditional NID-based naming

---

### Example 2: Data Center with Descriptive Names

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.datacenter.com boot_script_url=http://10.0.0.1 cache_duration=30s lease_duration=24h node_pattern="compute-{05d}" bmc_pattern="bmc-{05d}" domain=datacenter.com
```

**Result:**
- Nodes: `compute-00001.datacenter.com`, `compute-00042.datacenter.com`
- BMCs: `bmc-00001.datacenter.com`, `bmc-00042.datacenter.com`

**Use case:** Enterprise data center with descriptive hostnames and FQDN requirements

---

### Example 3: LANL Development Cluster

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.dev-osc.lanl.gov boot_script_url=http://172.16.0.253:8081 ca_cert=/etc/ssl/certs/lanl-ca.crt cache_duration=30s lease_duration=24h node_pattern="dev-s{02d}" bmc_pattern="bmc{03d}" domain=dev-osc.lanl.gov
```

**Result:**
- Nodes: `dev-s01.dev-osc.lanl.gov`, `dev-s42.dev-osc.lanl.gov`
- BMCs: `bmc001.dev-osc.lanl.gov`, `bmc042.dev-osc.lanl.gov`

**Use case:** Site-specific naming convention with short node IDs and institutional domain

---

### Example 4: Research Cluster with Project Prefix

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.research.edu boot_script_url=http://10.1.0.1 cache_duration=30s lease_duration=24h node_pattern="astro-node{04d}" bmc_pattern="astro-bmc{04d}" domain=astro.research.edu
```

**Result:**
- Nodes: `astro-node0001.astro.research.edu`, `astro-node0042.astro.research.edu`
- BMCs: `astro-bmc0001.astro.research.edu`, `astro-bmc0042.astro.research.edu`

**Use case:** Multi-tenant research facility with project-specific naming

---

### Example 5: Kubernetes Cluster

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.k8s.local boot_script_url=http://192.168.100.1 cache_duration=30s lease_duration=1h node_pattern="worker{03d}" domain=k8s.local
```

**Result:**
- Nodes: `worker001.k8s.local`, `worker042.k8s.local`
- BMCs: No hostname assigned

**Use case:** Kubernetes worker nodes with short lease times for dynamic environments

---

### Example 6: Xname-based Tracking

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h node_pattern="{id}" bmc_pattern="{id}" domain=cluster.local
```

**Result:**
- Nodes: `x3000c0s0b0n0.cluster.local`, `x3000c0s1b0n1.cluster.local`
- BMCs: `x3000c0s0b1.cluster.local`, `x3000c0s1b1.cluster.local`

**Use case:** Precise hardware tracking using SMD's native xname identifiers

---

### Example 7: Mixed Pattern (Advanced)

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h node_pattern="rack42-{id}" bmc_pattern="mgmt-{03d}" domain=lab.local
```

**Result:**
- Nodes: `rack42-x3000c0s0b0n0.lab.local`, `rack42-x3000c0s1b0n1.lab.local`
- BMCs: `mgmt-001.lab.local`, `mgmt-042.lab.local`

**Use case:** Combining rack information with xnames for nodes, simple numbering for BMCs

---

### Example 8: No Domain Suffix

**Configuration:**
```yaml
- coresmd: smd_url=https://smd.cluster.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h node_pattern="cn{04d}" bmc_pattern="ipmi{04d}"
```

**Result:**
- Nodes: `cn0001`, `cn0042`
- BMCs: `ipmi0001`, `ipmi0042`

**Use case:** Flat namespace without domain suffixes

---

## Common Patterns by Site Type

### Traditional HPC Center
```yaml
node_pattern="nid{04d}"
bmc_pattern=""
domain=""
```
Result: `nid0001`, `nid0002`, etc.

### Modern Data Center
```yaml
node_pattern="compute-{05d}"
bmc_pattern="bmc-{05d}"
domain="dc.example.com"
```
Result: `compute-00001.dc.example.com`, `bmc-00001.dc.example.com`

### Cloud/Kubernetes
```yaml
node_pattern="worker{03d}"
bmc_pattern=""
domain="k8s.local"
```
Result: `worker001.k8s.local`, `worker042.k8s.local`

### Research Lab
```yaml
node_pattern="{id}"
bmc_pattern="{id}"
domain="lab.university.edu"
```
Result: `x3000c0s0b0n0.lab.university.edu`

## Validation and Testing

After configuration, verify hostname generation:

1. Start CoreDHCP with your config
2. Check logs for hostname configuration:
   ```
   hostname config - node: dev-s{02d}, BMC: bmc{03d}, domain: dev-osc.lanl.gov
   ```
3. Request DHCP lease from a node
4. Verify assigned hostname in DHCP response (option 12)

## Troubleshooting

### No Hostname Assigned

**Problem:** Nodes/BMCs not receiving hostnames

**Solution:** 
- For nodes: Check that `node_pattern` is set (default: `nid{04d}`)
- For BMCs: Check that `bmc_pattern` is set (default: empty, no hostnames)
- Verify component type in SMD matches expectations

### Invalid Pattern Format

**Problem:** Error message about invalid pattern

**Solution:**
- Ensure patterns use `{Nd}` format where N is a digit (e.g., `{04d}`, not `{4d}`)
- Quote patterns with special characters in YAML: `node_pattern="my-node-{04d}"`

### Wrong Hostname Format

**Problem:** Hostnames don't match expected format

**Solution:**
- Review pattern syntax - ensure `{04d}` has two digits specifying padding
- Check for typos in pattern or domain parameters
- Verify NID values in SMD are correct

## Migration from Legacy Format

If you're using the legacy positional argument format (6 or 9 arguments), migrate to key=value format:

**Before (legacy 9-arg format):**
```yaml
- coresmd: https://smd.local http://192.168.1.1 "" 30s 24h false "dev-s{02d}" "bmc{03d}" dev-osc.lanl.gov
```

**After (key=value format):**
```yaml
- coresmd: smd_url=https://smd.local boot_script_url=http://192.168.1.1 cache_duration=30s lease_duration=24h node_pattern="dev-s{02d}" bmc_pattern="bmc{03d}" domain=dev-osc.lanl.gov
```

Benefits of key=value format:
- Self-documenting (parameter names are visible)
- Order-independent (parameters can appear in any order)
- Easier to maintain and modify
- Less prone to copy-paste errors
- Clear error messages for invalid configurations
