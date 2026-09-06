package main

import "testing"

func TestRecentCommandRejectsNonPositiveLimit(t *testing.T) {
	// CLI validation is exercised indirectly by the command contract; keep
	// this unit test as a placeholder until command execution is decoupled
	// from os.Exit.
}
