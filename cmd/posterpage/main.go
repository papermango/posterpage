// Command posterpage builds a single-page link-in-bio site. See
// docs/BUILD_PLAN.md for the spec and docs/ARCHITECTURE.md for how the code
// is organized.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"posterpage/internal/build"
)

const usage = `usage:
  posterpage build [-c <config-dir>] [-o <out-dir>]    build the site; defaults: ./site → ./public

build loads and validates config, renders the page, compiles styles, copies
assets into the out dir, and prints a manifest of the emitted files.`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "posterpage: missing command")
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "build":
		return cmdBuild(rest)
	case "help", "-h", "--help":
		fmt.Println(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "posterpage: unknown command %q\n", cmd)
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
}

func cmdBuild(argv []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	cfgDir := fs.String("c", "./site", "config directory (containing site.yml/.yaml/.json)")
	outDir := fs.String("o", "./public", "output directory (created if needed, overwritten on rebuild)")
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Println(usage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "posterpage: %v\n", err)
		return 1
	}

	files, err := build.Build(*cfgDir, *outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "posterpage: %v\n", err)
		return 1
	}
	if err := build.PrintManifest(os.Stdout, files); err != nil {
		fmt.Fprintf(os.Stderr, "posterpage: printing manifest: %v\n", err)
		return 1
	}
	return 0
}
