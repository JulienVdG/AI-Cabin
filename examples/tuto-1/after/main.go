// Package hello is the sample project for Tuto 1 of the cabin writing guide.
// It has no Dockerfile and no cabin files yet: the tutorial adds the cabin
// layer around it (see ../README.md).
package main

import (
	"fmt"
	"os"
)

func main() {
	greeting := "Hello from the sample Go project"
	if name := os.Getenv("HELLO_TARGET"); name != "" {
		greeting = fmt.Sprintf("Hello, %s", name)
	}
	fmt.Println(greeting)
}
