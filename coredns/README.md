# CoreSMD CoreDNS Plugin

The CoreSMD CoreDNS plugin provides dynamic DNS resolution for OpenCHAMI clusters by integrating with the State Management Database (SMD). This plugin enables automatic DNS record generation for compute nodes, BMCs, and other components managed by SMD.

## Features

- **Dynamic DNS Records**: Automatic A, PTR, CNAME, and TXT record generation
- **SMD Integration**: Real-time data from State Management Database
- **Multiple Record Types**: Support for forward and reverse DNS lookups
- **Component Metadata**: Rich TXT records with component information
- **Prometheus Metrics**: Built-in monitoring and metrics collection
- **Readiness Reporting**: Health checks and readiness endpoints
- **Cache Integration**: Shared cache with CoreDHCP plugins

## Installation

The CoreSMD CoreDNS plugin is included in the CoreSMD binary. No additional installation is required.

## Configuration

### Basic Configuration

```corefile
. {
    coresmd {
        smd_url https://smd.cluster.local
        ca_cert /path/to/ca.crt
        cache_duration 30s
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}
```

### Advanced Configuration with Custom Zones

```corefile
. {
    coresmd {
        smd_url https://smd.cluster.local
        ca_cert /path/to/ca.crt
        cache_duration 30s
        zone cluster.local {
            nodes nid{04d}
            bmcs bmc-{id}
        }
        zone test.local {
            nodes node{04d}
            bmcs mgmt-{id}
        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `smd_url` | string | required | SMD API endpoint URL |
| `ca_cert` | string | "" | Path to CA certificate for SMD TLS |
| `cache_duration` | duration | "30s" | Cache refresh interval |
| `zone` | block | auto | Zone configuration block |

### Zone Configuration

Each zone block supports the following options:

| Option | Type | Description |
|--------|------|-------------|
| `nodes` | string | Node hostname pattern (e.g., "nid{04d}") |
| `bmcs` | string | BMC hostname pattern (e.g., "bmc-{id}") |

## DNS Record Types

### A Records

Forward DNS resolution for nodes and BMCs:

```
nid0001.cluster.local.    IN A    192.168.1.10
bmc-bmc001.cluster.local. IN A    192.168.1.100
```

### PTR Records

Reverse DNS resolution:

```
10.1.168.192.in-addr.arpa. IN PTR nid0001.cluster.local.
100.1.168.192.in-addr.arpa. IN PTR bmc-bmc001.cluster.local.
```

### TXT Records

Component metadata:

```
nid0001.cluster.local. IN TXT "type=Node" "nid=1" "component_id=node001" "mac=00:11:22:33:44:55"
bmc-bmc001.cluster.local. IN TXT "type=NodeBMC" "component_id=bmc001" "mac=aa:bb:cc:dd:ee:ff"
```

### CNAME Records

Alias support (when configured):

```
alias.cluster.local. IN CNAME target.cluster.local.
```

## Monitoring

### Prometheus Metrics

The plugin exposes the following Prometheus metrics:

- `coredns_coresmd_requests_total` - Total DNS requests by type
- `coredns_coresmd_request_duration_seconds` - Request duration histogram
- `coredns_coresmd_cache_hits_total` - Cache hit count by record type
- `coredns_coresmd_cache_misses_total` - Cache miss count by record type
- `coredns_coresmd_smd_cache_age_seconds` - SMD cache age
- `coredns_coresmd_smd_cache_size` - SMD cache entry count

### Health Checks

The plugin implements readiness reporting:

- **Ready**: Returns true when SMD cache is populated and fresh
- **Health**: Returns true when the plugin is healthy

## Examples

### Basic DNS Server

```corefile
# Corefile
. {
    coresmd {
        smd_url https://smd.cluster.local
        cache_duration 30s
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}
```

### Multi-Zone Configuration

```corefile
# Corefile
. {
    coresmd {
        smd_url https://smd.cluster.local
        ca_cert /etc/ssl/certs/smd-ca.crt
        cache_duration 60s
        zone cluster.local {
            nodes nid{04d}
            bmcs bmc-{id}
        }
        zone mgmt.local {
            nodes mgmt{04d}
            bmcs ipmi-{id}
        }
    }
    prometheus 0.0.0.0:9153
    forward . 8.8.8.8
}
```

### Docker Deployment

```yaml
# docker-compose.yml
version: '3.8'
services:
  coredns:
    image: coresmd/coredns:latest
    ports:
      - "53:53/udp"
      - "53:53/tcp"
      - "9153:9153"
    volumes:
      - ./Corefile:/etc/coredns/Corefile
      - ./certs:/etc/ssl/certs
    command: ["-conf", "/etc/coredns/Corefile"]
```

### Kubernetes Deployment

```yaml
# coredns-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: coredns
  template:
    metadata:
      labels:
        app: coredns
    spec:
      containers:
      - name: coredns
        image: coresmd/coredns:latest
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        volumeMounts:
        - name: config-volume
          mountPath: /etc/coredns
        - name: certs-volume
          mountPath: /etc/ssl/certs
        args:
        - "-conf"
        - "/etc/coredns/Corefile"
      volumes:
      - name: config-volume
        configMap:
          name: coredns-config
      - name: certs-volume
        secret:
          secretName: smd-certs
```

## Troubleshooting

### Common Issues

1. **SMD Connection Failed**
   - Verify SMD URL is accessible
   - Check CA certificate path and validity
   - Ensure network connectivity

2. **No DNS Records Generated**
   - Check SMD cache is populated
   - Verify zone configuration matches SMD data
   - Review plugin logs for errors

3. **Cache Not Updating**
   - Verify cache duration setting
   - Check SMD API endpoint health
   - Review cache refresh logs

### Debug Mode

Enable debug logging:

```corefile
. {
    coresmd {
        smd_url https://smd.cluster.local
        cache_duration 30s
    }
    log
    prometheus 0.0.0.0:9153
}
```

### Health Check

Check plugin health:

```bash
# Check readiness
curl http://localhost:9153/ready

# Check metrics
curl http://localhost:9153/metrics | grep coresmd
```

## Integration with CoreDHCP

The CoreDNS plugin shares the same SMD cache infrastructure as the CoreDHCP plugins, ensuring consistency between DHCP leases and DNS records.

## Version Information

The plugin reports version information at startup:

```
INFO[0000] initializing coresmd/coredns v1.0.0 (clean), built 2024-01-01T00:00:00Z
```
