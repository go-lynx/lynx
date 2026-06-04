package main

// release is the current lynx tool version. Override at build time via ldflags:
//
//	go build -ldflags "-X 'main.release=v1.2.3'" ./cmd/lynx
var release = "dev"
