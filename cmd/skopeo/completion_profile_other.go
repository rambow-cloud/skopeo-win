//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func defaultPowerShellProfiles() ([]string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(config, "powershell", "Microsoft.PowerShell_profile.ps1")}, nil
}
func defaultCompletionDirectory() (string, error) {
	profiles, err := defaultPowerShellProfiles()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(profiles[0]), "SkopeoWin"), nil
}
func lockCompletionFile(file *os.File) (func(), error) { return func() {}, nil }
