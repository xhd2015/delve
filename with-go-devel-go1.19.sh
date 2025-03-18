#!/usr/bin/env bash

set -eo pipefail

export GOROOT=/Users/xhd2015/installed/go1.19.13
export PATH=$GOROOT/bin:$PATH

"$@"