package main

import (
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/image/v5/directory"
	"go.podman.io/image/v5/docker"
	dockerArchive "go.podman.io/image/v5/docker/archive"
	ociArchive "go.podman.io/image/v5/oci/archive"
	oci "go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/sif"
	"go.podman.io/image/v5/tarball"
	"go.podman.io/image/v5/transports"
)

func autocompleteImageNames(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	transport, details, haveTransport := strings.Cut(toComplete, ":")
	if !haveTransport {
		transports := supportedTransportSuggestions()
		return transports, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}
	switch transport {
	case ociArchive.Transport.Name(), dockerArchive.Transport.Name():
		// Can have [:{*reference|@source-index}]
		// FIXME: `oci-archive:/path/to/a.oci:<TAB>` completes paths
		return nil, cobra.ShellCompDirectiveNoSpace
	case sif.Transport.Name():
		return nil, cobra.ShellCompDirectiveDefault

	// Both directory and oci should have ShellCompDirectiveFilterDirs to complete only directories, but it doesn't currently work in bash: https://github.com/spf13/cobra/issues/2242
	case oci.Transport.Name():
		// Can have '[:{reference|@source-index}]'
		// FIXME: `oci:/path/to/dir/:<TAB>` completes paths
		return nil, cobra.ShellCompDirectiveDefault | cobra.ShellCompDirectiveNoSpace
	case directory.Transport.Name():
		return nil, cobra.ShellCompDirectiveDefault

	case docker.Transport.Name():
		if details == "" {
			return []cobra.Completion{transport + "://"}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
		}
	}
	return nil, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// supportedTransportSuggestions list all supported transports with the colon suffix.
func supportedTransportSuggestions() []string {
	tps := transports.ListNames()
	suggestions := make([]cobra.Completion, 0, len(tps))
	for _, tp := range tps {
		// ListNames is generally expected to filter out deprecated transports.
		// tarball: is not deprecated, but it is only usable from a Go caller (using tarball.ConfigUpdater),
		// so don’t offer it on the CLI.
		if tp != tarball.Transport.Name() {
			suggestions = append(suggestions, tp+":")
		}
	}
	return suggestions
}

func registerStaticCompletions(root *cobra.Command) {
	register := func(cmd *cobra.Command, flag string, choices []string) {
		if err := cmd.RegisterFlagCompletionFunc(flag, cobra.FixedCompletions(choices, cobra.ShellCompDirectiveNoFileComp)); err != nil {
			panic(err) // Duplicate registration is an internal programming error.
		}
	}
	register(root, "override-os", []string{"linux", "windows", "darwin", "freebsd", "openbsd"})
	register(root, "override-arch", []string{"amd64", "arm64", "arm", "386", "ppc64le", "s390x", "riscv64", "loong64"})
	for _, cmd := range root.Commands() {
		if cmd.Name() == "copy" {
			if err := cmd.RegisterFlagCompletionFunc("multi-arch", func(c *cobra.Command, args []string, partial string) ([]cobra.Completion, cobra.ShellCompDirective) {
				choices := []string{"system", "all", "index-only", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}
				prefix := ""
				if index := strings.LastIndex(partial, ","); index >= 0 {
					prefix = partial[:index+1]
					partial = partial[index+1:]
					choices = choices[3:]
				}
				matches := []cobra.Completion{}
				for _, choice := range choices {
					if strings.HasPrefix(choice, partial) {
						matches = append(matches, prefix+choice)
					}
				}
				return matches, cobra.ShellCompDirectiveNoFileComp
			}); err != nil {
				panic(err)
			}
		}
		if cmd.Name() == "sync" {
			register(cmd, "src", []string{"docker", "dir", "yaml"})
			register(cmd, "dest", []string{"docker", "dir"})
		}
		if cmd.Name() == "copy" || cmd.Name() == "sync" {
			register(cmd, "format", []string{"oci", "v2s1", "v2s2"})
			if cmd.Flags().Lookup("dest-compress-format") != nil {
				register(cmd, "dest-compress-format", []string{"gzip", "zstd", "zstd:chunked"})
			}
		}
	}
}
