// Command store-notices builds and checks notices for the Store package payload.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"go.podman.io/skopeo/internal/notices"
)

func main() {
	repository := flag.String("repository", ".", "source checkout containing vendor/modules.txt")
	layout := flag.String("layout", "", "package directory containing both Windows executables")
	goroot := flag.String("goroot", runtime.GOROOT(), "Go source tree matching the binary toolchain version")
	verify := flag.Bool("verify", false, "check all notice content and inventory against matching sources")
	flag.Parse()
	if *layout == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "-layout is required; positional arguments are not supported")
		os.Exit(1)
	}
	if err := notices.Run(*repository, *goroot, *layout, *verify); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Store third-party notice inventory and content verified against matching binary dependencies.")
}
