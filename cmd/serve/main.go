package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/iarkhanhelsky/serve/internal/cli"
	"github.com/iarkhanhelsky/serve/internal/config"
	"github.com/iarkhanhelsky/serve/internal/logview"
	"github.com/iarkhanhelsky/serve/internal/server"
	"github.com/iarkhanhelsky/serve/internal/types"
)

func main() {
	var (
		logMode    string
		errorsOnly bool
		logFile    string
	)

	flag.StringVar(&logMode, "log", "pretty", "log mode: pretty|compact|json")
	flag.BoolVar(&errorsOnly, "errors-only", false, "print only failed requests (status >= 400)")
	flag.StringVar(&logFile, "log-file", "", "path to raw access log file (default: temp file)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: serve [path] [listen|listen=upstream|listen:upstream]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Examples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  serve .\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  serve /path/to/dir\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  serve :8080\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  serve 80:8080\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  serve 0.0.0.0:80=127.0.0.1:8080\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	opts, err := cli.ParsePositionalArgs(flag.Args())
	if err != nil {
		exitf("parse args: %v", err)
	}
	if logMode != "pretty" && logMode != "compact" && logMode != "json" {
		exitf("invalid --log value: %s", logMode)
	}
	opts.LogMode = logMode
	opts.ErrorsOnly = errorsOnly

	root, err := cli.ResolveRoot(opts.Root)
	if err != nil {
		exitf("%v", err)
	}
	opts.Root = root
	opts.LogFile = logFile
	if opts.LogFile == "" {
		opts.LogFile = filepath.Join(os.TempDir(), "serve-access.log")
	}

	if err := os.WriteFile(opts.LogFile, nil, 0o644); err != nil {
		exitf("prepare log file: %v", err)
	}

	printBanner(opts)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := logview.Stream(ctx, opts.LogFile, opts); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "log stream error: %v\n", err)
		}
	}()

	cfg, err := config.BuildConfigJSON(opts, opts.LogFile)
	if err != nil {
		exitf("build caddy config: %v", err)
	}
	if err := server.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		exitf("serve failed: %v", err)
	}
}

func printBanner(opts types.RunOptions) {
	fmt.Printf("serve listening on %s\n", displayURL(opts.Listen))
	fmt.Printf("root: %s\n", opts.Root)
	if opts.Upstream != "" {
		fmt.Printf("proxy upstream: %s\n", opts.Upstream)
	}
	fmt.Printf("log mode: %s\n", opts.LogMode)
	fmt.Printf("raw access log: %s\n", opts.LogFile)
}

func displayURL(listen string) string {
	if strings.HasPrefix(listen, ":") {
		return "http://localhost" + listen
	}
	return "http://" + listen
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
