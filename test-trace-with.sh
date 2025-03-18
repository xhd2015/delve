#!/usr/bin/env bash

set -eo pipefail

# DO NOT MODIFY THIS FILE

project_root=$(git rev-parse --show-toplevel)

cd "$project_root"

# build dlv
go build -o dlv ./cmd/dlv
cp dlv $GOPATH/bin/dlv-with-xgo-trace

# build test binary
go build -gcflags="all=-N -l" -o __debug_bin_trace_with "$project_root/test/trace-with"

# run trace
dlv-with-xgo-trace trace 'main\..*' -e __debug_bin_trace_with --trace-with main.startTrace -- a b c