<!--
SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# CoreSMD Examples

This directory contains example configurations and deployment files for the CoreSMD project, which provides both CoreDHCP and CoreDNS plugins for OpenCHAMI clusters.

## Directory Structure

```
coredns/examples/
├── README.md                    # This file
├── basic/                       # Basic CoreDNS configurations
│   └── Corefile
├── advanced/                    # Advanced CoreDNS configurations
│   └── Corefile
└── kubernetes/                  # Kubernetes deployment examples
    ├── coredns-deployment.yaml
    └── coredns-configmap.yaml
```

## Quick Start

### CoreDNS

1. **Build the binary:**
   **NB** Builds must happen on a linux host in the root of the repository
   ```bash
   goreleaser build --snapshot --clean --single-target
   ```

2. **Run with basic configuration:**
   ```bash
   ./dist/coredns_linux_amd64_v1/coredns -conf coredns/examples/basic/Corefile
   ```

3. **Run with advanced configuration:**
   ```bash
   ./dist/linux_amd64/coredns_linux_amd64_v1/coredns -conf coredns/examples/advanced/Corefile
   ```

## Configuration Examples

#### Basic Configuration (`basic/Corefile`)

Simple DNS server configuration with:
- CoreSMD plugin for dynamic DNS records
- Prometheus metrics endpoint
- Forward DNS resolution

#### Advanced Configuration (`advanced/Corefile`)

Enhanced configuration with:
- Multiple DNS zones
- TLS certificate support
- Custom hostname patterns
- Extended cache duration

## Deployment Examples

### Kubernetes Deployment

Deploy to Kubernetes using the provided manifests:

```bash
kubectl apply -f coredns/examples/kubernetes/
```

## Customization

### Modifying SMD URL

Update the `smd_url` parameter in your configuration to point to your SMD instance:

```yaml
# CoreDNS
coresmd {
    smd_url https://your-smd-server.local
}
```

### Adding Custom Zones

Configure custom DNS zones in CoreDNS:

```corefile
coresmd {
    smd_url https://smd.cluster.local
    zone custom.local {
        nodes node{04d}
    }
}
```

### Enabling TokenSmith Auth for SMD Requests

The CoreDNS `coresmd` plugin can authenticate outbound SMD API requests using
TokenSmith-issued service tokens.

Auth directives in the `coresmd` block:

- `auth_mode`: `disabled` (default), `optional`, or `required`
- `tokensmith_url`: required when `auth_mode` is `optional` or `required`
- `refresh_before`: optional token refresh lead time (default `2m`)

Set the bootstrap token in the environment before starting CoreDNS:

```bash
export TOKENSMITH_BOOTSTRAP_TOKEN="<bootstrap-token>"
```

Example Corefile snippet:

```corefile
coresmd {
   smd_url https://smd.cluster.local
   auth_mode optional
   tokensmith_url https://tokensmith.cluster.local
   refresh_before 2m

   zone openchami.cluster {
      nodes nid{04d}
   }
}
```

`target_service` and `scopes` are not configured in the plugin. TokenSmith
reads those constraints from the bootstrap token claims.

## Testing

### Test DNS Resolution

```bash
# Test A record lookup (IPv4)
dig @localhost nid0001.cluster.local A

# Test AAAA record lookup (IPv6)
dig @localhost nid0001.cluster.local AAAA

# Test PTR record lookup (IPv4)
dig @localhost -x 192.168.1.10

# Test PTR record lookup (IPv6)
dig @localhost -x fd00:100::10

# Test TXT record lookup
dig @localhost TXT nid0001.cluster.local
```


### Check Metrics

```bash
# CoreDNS metrics
curl http://localhost:9153/metrics | grep coresmd

# Health check
curl http://localhost:9153/ready
```

## Troubleshooting

### Common Issues

1. **SMD Connection Failed**
   - Verify SMD URL is accessible
   - Check network connectivity
   - Validate CA certificate (if using TLS)

2. **No DNS Records Generated**
   - Check SMD cache is populated
   - Verify zone configuration
   - Review plugin logs
