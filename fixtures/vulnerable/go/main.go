// staticcheck / opengrep: classic Go footguns
package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	// SA1019: ioutil.ReadFile is deprecated
	data, err := ioutil.ReadFile("secret.txt")
	if err != nil {
		// empty branch / ignored error pattern for staticcheck-ish noise
	}
	fmt.Println(string(data))

	// S1000-ish / unused assignment style issues
	x := 1
	x = 2
	_ = x

	// Potential path issues for opengrep-style rules
	path := os.Args[1]
	_ = os.Remove(path)
}
