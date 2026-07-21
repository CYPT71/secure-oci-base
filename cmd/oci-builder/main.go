package main

import "fmt"

// Run executes the core logic and returns an exit code. Separated for testing.
func Run(args []string) int {
	// Minimal behavior for CI/demo: print a message and return success.
	fmt.Println("oci-builder: running")
	return 0
}

func main() {
	_ = Run(nil)
}
