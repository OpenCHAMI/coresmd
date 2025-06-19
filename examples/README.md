# CoreSMD Examples

This directory contains example configurations and deployment files for the CoreSMD project, which provides both CoreDHCP and CoreDNS plugins for OpenCHAMI clusters.

## Directory Structure

```
examples/
├── README.md                    # This file
├── coredhcp-config.yaml         # CoreDHCP configuration example
├── basic/                       # Basic CoreDNS configurations
│   └── Corefile
├── advanced/                    # Advanced CoreDNS configurations
│   └── Corefile
├── docker-compose.yml           # Docker Compose example
└── kubernetes/                  # Kubernetes deployment examples
    ├── coredns-deployment.yaml
    └── coredns-configmap.yaml
```

## Quick Start

### CoreDHCP

1. **Build the binary:**
   ```bash
   make build-coredhcp
   ```

2. **Run with example configuration:**
   ```bash
   ./build/coredhcp -c examples/coredhcp-config.yaml
   ```

### CoreDNS

1. **Build the binary:**
   ```bash
   make build-coredns
   ```

2. **Run with basic configuration:**
   ```bash
   ./build/coredns -conf examples/basic/Corefile
   ```

3. **Run with advanced configuration:**
   ```bash
   ./build/coredns -conf examples/advanced/Corefile
   ```

## Configuration Examples

### CoreDHCP Configuration

The `coredhcp-config.yaml` file demonstrates how to configure CoreDHCP with the CoreSMD plugin:

- **SMD Integration**: Connects to State Management Database for dynamic IP assignment
- **iPXE Support**: Provides boot script URLs for network booting
- **Cache Management**: Configurable cache duration for SMD data
- **Lease Management**: Configurable DHCP lease duration

### CoreDNS Configurations

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

### Docker Deployment

Use the provided `docker-compose.yml` to run CoreDNS in a container:

```bash
docker-compose up -d
```

### Kubernetes Deployment

Deploy to Kubernetes using the provided manifests:

```bash
kubectl apply -f examples/kubernetes/
```

## Customization

### Modifying SMD URL

Update the `smd_url` parameter in your configuration to point to your SMD instance:

```yaml
# CoreDHCP
- coresmd:
    - "https://your-smd-server.local"

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
        bmcs mgmt-{id}
    }
}
```

### TLS Configuration

For secure SMD connections, specify a CA certificate:

```yaml
# CoreDHCP
- coresmd:
    - "https://smd.cluster.local"
    - "http://192.168.1.1"
    - "/path/to/ca.crt"  # CA certificate path

# CoreDNS
coresmd {
    smd_url https://smd.cluster.local
    ca_cert /path/to/ca.crt
}
```

## Testing

### Test DNS Resolution

```bash
# Test A record lookup
dig @localhost nid0001.cluster.local

# Test PTR record lookup
dig @localhost -x 192.168.1.10

# Test TXT record lookup
dig @localhost TXT nid0001.cluster.local
```

### Test DHCP

```bash
# Test DHCP lease (requires DHCP client)
dhclient -v eth0
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

3. **DHCP Not Responding**
   - Check server binding
   - Verify plugin configuration
   - Review firewall settings

### Debug Mode

Enable debug logging by setting log level to `debug` in your configuration.

### Logs

Check application logs for detailed error messages and debugging information.

## Support

For issues and questions:
- Check the main project README
- Review the plugin documentation
- Open an issue on GitHub 