#!/usr/bin/env bash

set -eo pipefail

project_root=$(git rev-parse --show-toplevel)

if [[ ! -e "$project_root/go" ]]; then
    echo "the go that supports runtime.XgoGetCallerArgs() not found, clone with: "
    echo "  git clone https://github.com/xhd2015/go --branch trap"
    echo "  cd go/src"
    echo "  bash ./make.bash"
    exit 1
fi

export GOROOT=$project_root/go
export PATH=$GOROOT/bin:$PATH

"$@"