package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

func TestNewKeyFileDoesNotReplaceExistingObjects(t *testing.T) {
	for _, object := range []string{"file", "directory", "hard link"} {
		t.Run(object, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "key")
			victim := filepath.Join(dir, "unrelated")
			original := []byte("unrelated contents")
			if err := os.WriteFile(victim, original, 0o600); err != nil {
				t.Fatal(err)
			}
			switch object {
			case "file":
				if err := os.WriteFile(target, original, 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
			case "hard link":
				if err := os.Link(victim, target); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeNewKeyFile(target, []byte("new key material"), 0o600); err == nil || (object != "directory" && !errors.Is(err, fs.ErrExist)) {
				t.Fatalf("existing object was not safely rejected: %v", err)
			}
			for _, path := range []string{victim, target} {
				if path == target && object == "directory" {
					continue
				}
				b, err := os.ReadFile(path)
				if err != nil || string(b) != string(original) {
					t.Fatalf("existing contents changed at %s: %v", path, err)
				}
			}
		})
	}
}

func TestConcurrentKeyCreatorsHaveExactlyOneWinner(t *testing.T) {
	target := filepath.Join(t.TempDir(), "private-key")
	type outcome struct {
		data string
		err  error
	}
	results := make(chan outcome, 32)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := range 32 {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			<-start
			data := fmt.Sprintf("writer %d", i)
			results <- outcome{data, writeNewKeyFile(target, []byte(data), 0o600)}
		}(i)
	}
	close(start)
	group.Wait()
	close(results)
	winners := 0
	winner := ""
	for result := range results {
		if result.err == nil {
			winners++
			winner = result.data
		} else if !errors.Is(result.err, fs.ErrExist) {
			t.Fatalf("unexpected creator error: %v", result.err)
		}
	}
	data, err := os.ReadFile(target)
	if err != nil || winners != 1 || string(data) != winner {
		t.Fatalf("creation was not exclusive: winners=%d read=%v", winners, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private key mode incorrect: %v", err)
		}
	}
}

func TestDanglingKeyLinkIsRefusedWithoutTouchingItsTarget(t *testing.T) {
	dir := t.TempDir()
	link, target := filepath.Join(dir, "key"), filepath.Join(dir, "outside-key")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
			t.Skip("Windows symlink privilege unavailable; hard-link and executable race checks still run")
		}
		t.Fatal(err)
	}
	if err := ensurePathDoesNotExist(link); err == nil {
		t.Fatal("dangling link passed preflight")
	}
	if err := writeNewKeyFile(link, []byte("secret"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("dangling link accepted: %v", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dangling target was touched: %v", err)
	}
}
