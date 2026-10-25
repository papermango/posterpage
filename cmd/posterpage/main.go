// Command posterpage builds and (eventually) serves a single-page
// link-in-bio site. See docs/BUILD_PLAN.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"posterpage/internal/config"
)

const usage = `usage:
  posterpage build [-c <config-dir>] [-o <out-dir>]    build the site; defaults: ./site → ./public
  posterpage serve [-o <out-dir>] [-addr <addr>]       serve the built site (not implemented yet)

For this milestone, build loads and validates the config and prints it as
JSON; no output files are written yet.`

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
	case "serve":
		fmt.Fprintln(os.Stderr, "posterpage: serve is not implemented yet (planned for a later milestone)")
		return 1
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
	outDir := fs.String("o", "./public", "output directory (unused until the page-render milestone)")
	_ = *outDir // accepted now; used by page rendering in a later milestone
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Println(usage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "posterpage: %v\n", err)
		return 1
	}

	cfg, name, err := config.Load(*cfgDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "posterpage: %v\n", err)
		return 1
	}
	if err := config.Validate(cfg, *cfgDir, name); err != nil {
		fmt.Fprintf(os.Stderr, "posterpage: %v\n", err)
		return 1
	}
	if err := config.PrintJSON(os.Stdout, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "posterpage: printing config: %v\n", err)
		return 1
	}
	return 0
}
