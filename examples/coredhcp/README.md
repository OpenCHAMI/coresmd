# CoreDHCP Configuration Examples for CoreSMD

This directory contains example CoreDHCP configurations for the CoreSMD plugin.

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

See [coredhcp.yaml](coredhcp.yaml) for a full example with documentation comments.
