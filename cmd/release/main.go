// Command release validates manual version tags and publishes skopeo-win releases through gh.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"

	"go.podman.io/skopeo/internal/release"
)

func run() error {
	mode := flag.String("mode", "plan", "plan or publish")
	version := flag.String("version", "", "independent version to publish")
	tag := flag.String("tag", "", "existing maintainer-created stable tag for planning")
	assets := flag.String("assets", "dist", "directory containing the four release files")
	notesFile := flag.String("notes-file", "docs/release-notes.md", "reviewed Markdown release summary")
	output := flag.String("output", "", "append version and tag to a GitHub Actions output file")
	preview := flag.Bool("preview", false, "produce a development version without accessing GitHub")
	flag.Parse()
	commit := os.Getenv("GITHUB_SHA")
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) {
		return fmt.Errorf("GITHUB_SHA must be a full commit SHA")
	}
	m := release.Manager{Run: release.GH, Repo: os.Getenv("GITHUB_REPOSITORY"), Commit: commit, RunID: os.Getenv("GITHUB_RUN_ID")}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(m.Repo) || !regexp.MustCompile(`^\d+$`).MatchString(m.RunID) {
		return fmt.Errorf("valid GITHUB_REPOSITORY and GITHUB_RUN_ID are required")
	}
	switch *mode {
	case "plan":
		resolved := "0.0.0-dev." + commit[:12]
		if !*preview {
			var err error
			resolved, err = m.PlanTag(*tag)
			if err != nil {
				return err
			}
		}
		fmt.Println(resolved)
		if *output != "" {
			f, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = fmt.Fprintf(f, "version=%s\ntag=v%s\n", resolved, resolved)
			return err
		}
		return nil
	case "publish":
		summary, err := os.ReadFile(*notesFile)
		if err != nil {
			return fmt.Errorf("reading release summary: %w", err)
		}
		m.Summary = string(summary)
		return m.Publish(*version, *assets)
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
