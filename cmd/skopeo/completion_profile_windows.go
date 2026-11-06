package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Find the calling console host, including launchers between it and this process.
// Custom hosts can select their profile explicitly with --profile $PROFILE.
func parentPowerShell() string {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snapshot)
	entries := map[uint32]windows.ProcessEntry32{}
	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err := windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		entries[entry.ProcessID] = entry
	}
	pid := uint32(os.Getppid())
	for depth := 0; depth < 12 && pid != 0; depth++ {
		process, ok := entries[pid]
		if !ok {
			return ""
		}
		name := strings.ToLower(windows.UTF16ToString(process.ExeFile[:]))
		if name == "powershell.exe" || name == "pwsh.exe" {
			return name
		}
		pid = process.ParentProcessID
	}
	return ""
}

func defaultPowerShellProfiles() ([]string, error) {
	docs, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DONT_VERIFY)
	if err != nil {
		return nil, err
	}
	folder := "PowerShell"
	host := parentPowerShell()
	if host == "powershell.exe" {
		folder = "WindowsPowerShell"
	} else if host == "" {
		if _, err := exec.LookPath("pwsh.exe"); err != nil {
			folder = "WindowsPowerShell"
		}
	}
	return []string{filepath.Join(docs, folder, "Microsoft.PowerShell_profile.ps1")}, nil
}

func defaultCompletionDirectory() (string, error) {
	docs, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DONT_VERIFY)
	if err != nil {
		return "", err
	}
	return filepath.Join(docs, "PowerShell", "SkopeoWin"), nil
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
