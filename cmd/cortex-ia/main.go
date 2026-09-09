package main

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/app"
)

// version is set by GoReleaser via ldflags at build time (-X main.version=...).
// In local development or source builds, it defaults to "dev" and falls back to git describe.
var version = "dev"

func main() {
	app.Version = app.ResolveVersion(version)
	if app.Version == "dev" {
		if gitVer := app.GitDescribeVersion(); gitVer != "" {
			app.Version = gitVer
		}
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
