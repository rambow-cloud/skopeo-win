//go:build !windows

package main

import "runtime"

func defaultImageArchitecture() string {
	return runtime.GOARCH
}
