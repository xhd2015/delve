#!/usr/bin/env bash

set -eo pipefail

# DO NOT MODIFY THIS FILE

project_root=$(git rev-parse --show-toplevel)

cd "$project_root"

go build -o dlv ./cmd/dlv

go build -gcflags="all=-N -l" -o __debug_bin_auto_trap "$project_root/test/auto-trap"

# go test -c -o "$project_root/pkg/proc/__debug_bin_test" -gcflags="all=-N -l" "$project_root/pkg/proc"

# ./dlv exec --auto-trap --init auto-trap.dlv ./__debug_bin_auto_trap
./dlv trace 'main.*' -e ./__debug_bin_auto_trap