package main

import (
	"debug/pe"
	"runtime"

	"golang.org/x/sys/windows"
)

func defaultImageArchitecture() string {
	var processMachine, nativeMachine uint16
	if err := windows.IsWow64Process2(windows.CurrentProcess(), &processMachine, &nativeMachine); err != nil {
		return runtime.GOARCH
	}
	return imageArchitectureForMachine(nativeMachine)
}

func imageArchitectureForMachine(machine uint16) string {
	switch machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "amd64"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64"
	default:
		return runtime.GOARCH
	}
}
