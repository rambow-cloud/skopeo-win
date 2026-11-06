package main

import (
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage/pkg/configfile"
)

// Match the upstream permissive default for all image sources.
// Configured policies remain authoritative; TLS and authentication are unchanged.
//
//go:embed windows-policy.json
var defaultWindowsPolicy []byte

func loadDefaultImagePolicy(sys *types.SystemContext, writeDefault bool) (*signature.Policy, error) {
	policy, err := signature.DefaultPolicy(sys)
	if err == nil || runtime.GOOS != "windows" || !errors.Is(err, configfile.ErrConfigFileNotFound) {
		return policy, err
	}
	if appeared, err := readAppearedPolicy(sys); appeared != nil || err != nil {
		return appeared, err
	}
	if writeDefault {
		directory, pathErr := configfile.UserConfigPath()
		if pathErr == nil {
			pathErr = writeDefaultImagePolicy(directory)
		}
		// Another process or administrator may have provided a policy meanwhile.
		// Honor it, including parse/read failures, even when our save failed.
		policy, err = signature.DefaultPolicy(sys)
		if err == nil || !errors.Is(err, configfile.ErrConfigFileNotFound) {
			return policy, err
		}
		if appeared, err := readAppearedPolicy(sys); appeared != nil || err != nil {
			return appeared, err
		}
		logrus.Debugf("Using embedded image trust policy; default policy save result: %v", pathErr)
	}
	return signature.NewPolicyFromBytes(defaultWindowsPolicy)
}

// A dangling link is a configured but unavailable policy, not permission to
// replace its contents or silently apply a different trust policy.
func readAppearedPolicy(sys *types.SystemContext) (*signature.Policy, error) {
	conf := &configfile.File{Name: "policy", Extension: "json", DoNotLoadDropInFiles: true, EnvironmentName: "CONTAINERS_POLICY_JSON"}
	if sys != nil {
		conf.RootForImplicitAbsolutePaths = sys.RootForImplicitAbsolutePaths
	}
	paths, err := configfile.GetSearchPaths(conf)
	if err != nil {
		return nil, err
	}
	for _, path := range paths.MainFiles {
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			// Re-read a concurrently published file rather than ignoring it.
			return signature.DefaultPolicy(sys)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("checking image trust policy %q: %w", path, err)
		}
	}
	return nil, nil
}

func writeDefaultImagePolicy(directory string) error {
	if !filepath.IsAbs(directory) {
		return errors.New("APPDATA does not identify an absolute configuration directory")
	}
	parent, name := filepath.Split(filepath.Clean(directory))
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	configRoot, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer configRoot.Close()
	temporary := ".policy-" + rand.Text() + ".tmp"
	f, err := configRoot.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer configRoot.Remove(temporary)
	_, writeErr := f.Write(defaultWindowsPolicy)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	// A same-directory hard link publishes complete bytes atomically and fails
	// if a policy (including a link) already exists. Never replace that entry.
	if err := configRoot.Link(temporary, "policy.json"); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}
