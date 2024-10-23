# coresmd

A CoreDHCP plugin with a pull-through cache that uses
[SMD](https://github.com/OpenCHAMI/smd) as its source of truth.

## Building

This is meant to be built statically into
[CoreDHCP](https://github.com/coredhcp/coredhcp) using the
[coredhcp-generator](https://github.com/coredhcp/coredhcp/blob/master/cmds/coredhcp-generator).

### Container

This repository includes a Dockerfile that builds CoreDHCP with its core plugins
as well as this plugin.

```
docker build . --tag coresmd:latest
```

### Bare Metal

**NOTE:** Certain source files in CoreDHCP only build on Linux, which will cause
build errors when building on other platforms like Mac.

It is recommended to do this within a clean directory.

1. Create directory for generated source files:

   ```
   mkdir coresmd
   ```

1. Clone CoreDHCP and change the working directory to the coredhcp-generator
   tool.

   ```
   git clone https://github.com/coredhcp/coredhcp
   cd coredhcp/cmds/coredhcp-generator
   ```

1. Build the generator.

   ```
   go mod download
   go build
   ```

1. Run the generator to generate the CoreDHCP source file.

   ```
   ./coredhcp-generator -f core-plugins.txt -t coredhcp.go.template -o ../../../coresmd/coredhcp.go \
     github.com/synackd/coresmd/coresmd \
     github.com/synackd/coresmd/bootloop
   ```

   You should see output similar to the following:

   ```
   2024/10/23 22:57:51 Generating output file '../../../coresmd/coredhcp.go' with 17 plugin(s):
   2024/10/23 22:57:51   1) github.com/synackd/coresmd/bootloop
   2024/10/23 22:57:51   2) github.com/coredhcp/coredhcp/plugins/autoconfigure
   2024/10/23 22:57:51   3) github.com/coredhcp/coredhcp/plugins/file
   2024/10/23 22:57:51   4) github.com/coredhcp/coredhcp/plugins/prefix
   2024/10/23 22:57:51   5) github.com/coredhcp/coredhcp/plugins/leasetime
   2024/10/23 22:57:51   6) github.com/coredhcp/coredhcp/plugins/mtu
   2024/10/23 22:57:51   7) github.com/coredhcp/coredhcp/plugins/nbp
   2024/10/23 22:57:51   8) github.com/coredhcp/coredhcp/plugins/router
   2024/10/23 22:57:51   9) github.com/coredhcp/coredhcp/plugins/searchdomains
   2024/10/23 22:57:51  10) github.com/coredhcp/coredhcp/plugins/dns
   2024/10/23 22:57:51  11) github.com/coredhcp/coredhcp/plugins/netmask
   2024/10/23 22:57:51  12) github.com/coredhcp/coredhcp/plugins/range
   2024/10/23 22:57:51  13) github.com/coredhcp/coredhcp/plugins/serverid
   2024/10/23 22:57:51  14) github.com/coredhcp/coredhcp/plugins/sleep
   2024/10/23 22:57:51  15) github.com/coredhcp/coredhcp/plugins/staticroute
   2024/10/23 22:57:51  16) github.com/synackd/coresmd/coresmd
   2024/10/23 22:57:51  17) github.com/coredhcp/coredhcp/plugins/ipv6only
   2024/10/23 22:57:51 Generated file '../../../coresmd/coredhcp.go'. You can build it by running 'go build' in the output directory.
   ../../../coresmd
   ```

1. Change directory into the directory, initialize it as a Go module.

   ```
   cd ../../../coresmd
   go mod init coredhcp   # the module name doesn't matter
   go mod edit -go=1.21
   go mod edit -replace=github.com/coredhcp/coredhcp=../coredhcp
   ```
   **Only if you have coresmd checked out locally**
   ```
   go mod edit -replace=github.com/synackd/coresmd=<path_to_checkout>
   ```
   Finally:
   ```
   go mod tidy
   ```

1. Build CoreDHCP.

   ```
   go build
   ```

You'll now have a `coredhcp` binary in the current directory you can run.

## Configuration

CoreDHCP requires a config file to run. An example `config.yaml` that configures
the basics along with coresmd is as follows:

```yaml
server4:
  plugins:
    - lease_time: 3600s
    - server_id: 172.16.0.253
    - dns: 10.15.3.42 10.0.69.16 10.0.69.17
    - router: 172.16.0.254
    - netmask: 255.255.255.0
    - coresmd: https://foobar.openchami.cluster http://172.16.0.253:8081 /root_ca/root_ca.crt 30s eyJh...
```

For the coresmd plugin, the arguments are as follows:

1. **OpenCHAMI Base URL** --- This is the base URL for which the plugin will
   append OpenCHAMI service endpoints onto.
1. **Boot Script Base URL** --- Since the OpenCHAMI Base URL is usually
   protected with TLS, this argument represents the base URL that nodes are told
   to fetch their boot script from. Since it is likely that the CA certificate
   is not in the bootloader, this URL is usually HTTP without TLS.
1. **CA Certificate Path** --- This argument is the path to a CA certificate (in
   PEM format) to validate OpenCHAMI certificates with. This argument can be
   empty to use the system certificate store.
1. **Cache Validity Duration** --- This argument represents the duration that
   the cache should remain valid before updating. This is a duration string that
   is parsed by [`time.ParseDuration()`](https://pkg.go.dev/time#ParseDuration)
   (e.g. `1h`, `1.5h`, `1h30m`). Valid time units are "ns", "us" (or "µs"),
   "ms", "s", "m", "h".
1. **SMD JWT** --- This plugin reads SMD's `/Inventory/RedfishEndpoints`
   endpoint (which is currently protected) to get BMC info. This argument is a
   valid JSON Web Token (JWT) to present to SMD to authenticate to this
   endpoint. It is hoped that this endpoint will open up to eliminate the need
   for this argument.
