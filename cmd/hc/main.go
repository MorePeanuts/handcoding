// Command hc is the handcoding CLI: a local judge for LeetCode-style problems.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MorePeanuts/handcoding/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "hc:", msg)
		}
		os.Exit(1)
	}
}
