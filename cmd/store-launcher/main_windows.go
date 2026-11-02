// The Store application entry opens a persistent usage window. Execution aliases
// bypass this launcher and invoke skopeo.exe directly.
package main

import (
	"bufio"
	"fmt"
	"os"

	"go.podman.io/skopeo/version"
)

func main() {
	fmt.Printf("skopeo-win %s\n\n", version.Version)
	fmt.Println("Inspect, copy and synchronize container images from the Windows terminal.")
	fmt.Println("Open PowerShell, Command Prompt, or Windows Terminal and run:")
	fmt.Println()
	fmt.Println("  skopeo-win --help")
	fmt.Println("  skopeo-win --version")
	fmt.Println()
	fmt.Println("The skopeo command is also available when its app execution alias is enabled.")
	fmt.Println("Documentation: https://github.com/rambow-cloud/skopeo-win")
	fmt.Println()
	fmt.Print("Press Enter to close this window...")
	// Enter closes an interactive window; EOF also lets non-interactive callers exit.
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
