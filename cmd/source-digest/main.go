package main

import (
 "fmt"
 "log"
 "os"
 "go.podman.io/skopeo/internal/notices"
)

func main() {
 if len(os.Args) != 2 { log.Fatal("usage: source-digest DIRECTORY") }
 value, err := notices.SourceDigest(os.Args[1])
 if err != nil { log.Fatal(err) }
 fmt.Println(value)
}
