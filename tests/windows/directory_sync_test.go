package windows_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySyncWindowsPaths(t *testing.T) {
	for _, name := range []string{"absolute", "relative", "trailing separator", "forward slashes", "nested scoped", "nested unscoped"} {
		t.Run(name, func(t *testing.T) {
			c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
			input := filepath.Join(c.root, "synced")
			source := input
			want := "synced"
			scoped := false
			switch name {
			case "relative":
				source = "synced"
			case "trailing separator":
				source += string(os.PathSeparator)
			case "forward slashes":
				source = filepath.ToSlash(source)
			case "nested scoped", "nested unscoped":
				source = filepath.Join(c.root, "images")
				input = filepath.Join(source, "team", "synced")
				if name == "nested scoped" {
					scoped = true
					want = "team/synced"
				}
			}
			copyFixtureDirectory(t, f.dir, input)
			args := []string{"sync", "--all", "--remove-signatures", "--src=dir", "--dest=docker", "--dest-tls-verify=false"}
			if scoped {
				args = append(args, "--scoped")
			}
			c.run(t, "", append(args, source, host+"/mirror")...)
			i := getIndex(t, c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/mirror/"+want+":latest"), 2)
			if i.Manifests[0].Digest != f.instances[0].Digest {
				t.Fatal("directory sync changed the selected instance")
			}
		})
	}
}
