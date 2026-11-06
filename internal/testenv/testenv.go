// Package testenv decides what a test does when a tool it needs (go,
// python3, git) is missing or below the version hc supports: it skips, so
// that contributors without the tool can still run go test, unless
// $HC_TEST_REQUIRE_TOOLS is set, as make test and CI set it, which makes it
// fail instead, so that a test environment that lacks a tool cannot pass
// without running the tests that need it.
package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// RequireTools is the environment variable that turns the skips of this
// package into failures when it is true (1, true).
const RequireTools = "HC_TEST_REQUIRE_TOOLS"

// Required reports whether $HC_TEST_REQUIRE_TOOLS is set to true.
func Required() bool {
	ok, _ := strconv.ParseBool(os.Getenv(RequireTools))
	return ok
}

// Skip skips t because a tool it needs is unavailable, the reason given by
// args as by fmt.Sprint, or fails it when Required.
func Skip(t testing.TB, args ...any) {
	t.Helper()
	reason := fmt.Sprint(args...)
	if Required() {
		t.Fatalf("%s (%s is set)", reason, RequireTools)
	}
	t.Skip(reason)
}

// LookPath returns the path of tool on PATH, and Skips t without it.
func LookPath(t testing.TB, tool string) string {
	t.Helper()
	path, err := exec.LookPath(tool)
	if err != nil {
		Skip(t, tool+" not in PATH")
	}
	return path
}
