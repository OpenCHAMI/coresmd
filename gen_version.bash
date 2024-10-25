#!/bin/bash
base_dir="$(readlink -f $(dirname ${BASH_SOURCE[0]}))"
version_file=internal/version/version.go
cd "$base_dir"
sed -i "s/v0.0.0/$(git describe --tags --always --dirty --broken --abbrev=0)/" $version_file
sed -i "s/0000000/$(git rev-parse --short HEAD)/" $version_file
sed -i "s/0000-00-00:00:00:00/$(date -Iseconds)/" $version_file
