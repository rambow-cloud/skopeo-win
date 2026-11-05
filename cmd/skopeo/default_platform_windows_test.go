package main

import (
	"debug/pe"
	"runtime"
	"testing"
)

func TestNativeMachineArchitecture(t *testing.T) {
	for _, tc := range []struct {
		machine uint16
		want    string
	}{
		{pe.IMAGE_FILE_MACHINE_AMD64, "amd64"},
		{pe.IMAGE_FILE_MACHINE_ARM64, "arm64"},
		{0, runtime.GOARCH},
	} {
		if got := imageArchitectureForMachine(tc.machine); got != tc.want {
			t.Fatalf("native machine %x: got %s, want %s (process architecture %s)", tc.machine, got, tc.want, runtime.GOARCH)
		}
	}
}
