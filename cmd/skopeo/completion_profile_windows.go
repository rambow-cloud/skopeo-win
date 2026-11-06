package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func completionAutoSupported() bool { return true }

func automaticPowerShellProfiles(profiles []string) []string {
	if os.Getenv("SKOPEO_POWERSHELL_PROFILES") != "" {
		return profiles
	}
	selected := []string{}
	for _, profile := range profiles {
		data, err := os.ReadFile(profile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if bytes.Contains(data, encodedProfileText(data, "# SIG # Begin signature block")) {
			continue
		}
		if bytes.Contains(data, encodedProfileText(data, completionStart)) {
			selected = append(selected, profile)
			continue
		}
		shell := "pwsh.exe"
		if strings.EqualFold(filepath.Base(filepath.Dir(profile)), "WindowsPowerShell") {
			system, err := windows.GetSystemDirectory()
			if err != nil {
				continue
			}
			shell = filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		command := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-Command", "[string](Get-ExecutionPolicy)")
		for _, item := range os.Environ() {
			key, _, _ := strings.Cut(item, "=")
			if !strings.EqualFold(key, "PSExecutionPolicyPreference") {
				command.Env = append(command.Env, item)
			}
		}
		result, err := command.Output()
		cancel()
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(string(result))) {
		case "bypass", "unrestricted", "remotesigned":
			selected = append(selected, profile)
		}
	}
	return selected
}
func defaultPowerShellProfiles() ([]string, error) {
	documents, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(documents, "PowerShell", "profile.ps1"), filepath.Join(documents, "WindowsPowerShell", "profile.ps1")}, nil
}
func lockCompletionFile(file *os.File) (func(), error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return nil, err
	}
	if info.NumberOfLinks > 1 {
		return nil, errors.New("refusing a hard-linked completion/profile file")
	}
	overlapped := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 0xffffffff, 0xffffffff, overlapped); err != nil {
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 0xffffffff, 0xffffffff, overlapped) }, nil
}
