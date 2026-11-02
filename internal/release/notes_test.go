package release

import (
	"strings"
	"testing"
)

func TestNotesPreserveReviewedContentAndProvenance(t *testing.T) {
	m := Manager{Repo: "owner/repo", Commit: testCommit, RunID: "123", Summary: "\n## Features\n\n- Use `--download-foreign-layers`.\n\n## Verification\n\nRegistry push/pull passed.\n"}
	notes, err := m.Notes("1.0.2")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{marker("123"), "# skopeo-win v1.0.2", strings.TrimSpace(m.Summary), "/commit/" + testCommit, "/actions/runs/123", "not Authenticode signed", "not runtime-tested"} {
		if !strings.Contains(notes, part) {
			t.Errorf("missing %q in notes", part)
		}
	}
}

func TestMissingSummaryPreventsPublicationMutations(t *testing.T) {
	for _, summary := range []string{"", " \n\t"} {
		called := false
		m := Manager{Run: func(args ...string) ([]byte, error) { called = true; return nil, nil }, Repo: "owner/repo", Commit: testCommit, RunID: "123", Summary: summary}
		if err := m.Publish("1.0.2", t.TempDir()); err == nil || !strings.Contains(err.Error(), "summary") {
			t.Fatalf("expected summary error, got %v", err)
		}
		if called {
			t.Fatal("publication mutated GitHub without a summary")
		}
	}
}
