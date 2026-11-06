//go:build !windows

package main

import (
	"errors"
	"os"
)

func completionAutoSupported() bool                          { return false }
func automaticPowerShellProfiles(profiles []string) []string { return nil }
func defaultPowerShellProfiles() ([]string, error) {
	return nil, errors.New("automatic PowerShell profile discovery is supported on Windows; use --profile for an explicit path")
}
func lockCompletionFile(file *os.File) (func(), error) { return func() {}, nil }
