package main

import "runtime/debug"

// release is overridden at build time:
//
//	go build -ldflags "-X 'main.release=v1.2.3'" ./cmd/lynx
//
// When installed via "go install", version() falls back to the module version
// embedded in the binary by the Go toolchain.
var release = "dev"

func version() string {
	if release != "dev" {
		return release
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return release
}
