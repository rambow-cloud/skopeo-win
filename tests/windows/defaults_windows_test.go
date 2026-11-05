package windows_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDefaultPolicyCreationCannotEscapeThroughJunction(t *testing.T) {
	c, ref, _ := publicFixture(t)
	outside := t.TempDir()
	link := filepath.Join(c.root, "containers")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; $null=New-Item -ItemType Junction -Path $env:POLICY_TEST_LINK -Target $env:POLICY_TEST_TARGET")
	cmd.Env = append(os.Environ(), "POLICY_TEST_LINK="+link, "POLICY_TEST_TARGET="+outside)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v: %s", err, out)
	}
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil {
			t.Error(err)
		}
	})
	// The default file cannot safely be saved, so the same embedded policy is used.
	c.run(t, "", "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("policy bootstrap wrote through a junction: %v %v", entries, err)
	}
}
