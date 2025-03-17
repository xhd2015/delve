module github.com/xhd2015/xgo

go 1.21

toolchain go1.23.6

require github.com/go-delve/delve v1.25.1

require (
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	golang.org/x/arch v0.11.0 // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/telemetry v0.0.0-20241106142447-58a1122356f5 // indirect
)

replace github.com/go-delve/delve => ../
