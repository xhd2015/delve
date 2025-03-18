#!/usr/bin/env bash

set -eo pipefail

# DO NOT MODIFY THIS FILE

project_root=$(git rev-parse --show-toplevel)

(cd ./go/test/trap/macho/example && go build -o "$project_root/pkg/proc/__debug_bin_example" -gcflags="all=-N -l" ./)

cd "$project_root"

./with-go-devel.sh go test -c -o "$project_root/pkg/proc/__debug_bin_test" -gcflags="all=-N -l" "$project_root/pkg/proc"

cd "$project_root/pkg/proc"

# each arg add -test., if starts with -, add -test.
args=("$@")
n=${#args[@]}
for((i=0;i<n;i++)); do
    arg=${args[$i]}
    if [[ "$arg" = -* ]]; then
        args[$i]="-test.${arg:1}"
    fi
done

./__debug_bin_test "${args[@]}"