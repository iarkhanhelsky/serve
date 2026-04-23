package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/iarkhanhelsky/serve/internal/types"
)

var barePortPattern = regexp.MustCompile(`^\d+$`)

// ParsePositionalArgs parses positional arguments into normalized runtime options.
func ParsePositionalArgs(args []string) (types.RunOptions, error) {
	opts := types.RunOptions{
		Root:    ".",
		Listen:  ":8000",
		LogMode: "pretty",
	}

	if len(args) > 2 {
		return opts, fmt.Errorf("expected up to 2 positional args, got %d", len(args))
	}

	if len(args) == 0 {
		return opts, nil
	}

	items := make([]classifiedArg, 0, len(args))
	for _, a := range args {
		c, err := classifyArg(a)
		if err != nil {
			return opts, err
		}
		items = append(items, c)
	}

	for _, item := range items {
		switch item.kind {
		case argPath:
			if opts.Root != "." {
				return opts, errors.New("multiple paths provided")
			}
			opts.Root = item.value
		case argListen:
			if opts.Listen != ":8000" || opts.Upstream != "" {
				return opts, errors.New("multiple listen/proxy args provided")
			}
			opts.Listen = item.value
		case argPair:
			if opts.Listen != ":8000" || opts.Upstream != "" {
				return opts, errors.New("multiple listen/proxy args provided")
			}
			opts.Listen = item.listen
			opts.Upstream = item.upstream
		}
	}

	return opts, nil
}

// ResolveRoot normalizes root path to an absolute path and validates it exists.
func ResolveRoot(root string) (string, error) {
	p, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root path: %w", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("stat root path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root path is not a directory: %s", p)
	}
	return p, nil
}

type argKind string

const (
	argPath   argKind = "path"
	argListen argKind = "listen"
	argPair   argKind = "pair"
)

type classifiedArg struct {
	kind     argKind
	value    string
	listen   string
	upstream string
}

func classifyArg(raw string) (classifiedArg, error) {
	if strings.Contains(raw, "=") {
		l, u, err := parsePair(raw)
		if err != nil {
			return classifiedArg{}, err
		}
		return classifiedArg{kind: argPair, listen: l, upstream: u}, nil
	}

	if pairAliasPattern(raw) {
		l, u, err := parsePairAlias(raw)
		if err != nil {
			return classifiedArg{}, err
		}
		return classifiedArg{kind: argPair, listen: l, upstream: u}, nil
	}

	if looksLikeListen(raw) {
		l, err := normalizeListen(raw)
		if err != nil {
			return classifiedArg{}, err
		}
		return classifiedArg{kind: argListen, value: l}, nil
	}

	return classifiedArg{kind: argPath, value: raw}, nil
}

func parsePair(raw string) (string, string, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid pair syntax: %s", raw)
	}
	listen, err := normalizeListen(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid listen address in %q: %w", raw, err)
	}
	upstream, err := normalizeUpstream(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid upstream in %q: %w", raw, err)
	}
	return listen, upstream, nil
}

func parsePairAlias(raw string) (string, string, error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid alias syntax: %s", raw)
	}
	listen, err := normalizeListen(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid listen alias in %q: %w", raw, err)
	}
	upstream, err := normalizeUpstream(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid upstream alias in %q: %w", raw, err)
	}
	return listen, upstream, nil
}

func pairAliasPattern(raw string) bool {
	parts := strings.Split(raw, ":")
	return len(parts) == 2 && barePortPattern.MatchString(parts[0]) && barePortPattern.MatchString(parts[1])
}

func looksLikeListen(raw string) bool {
	if strings.HasPrefix(raw, ":") {
		return true
	}
	if barePortPattern.MatchString(raw) {
		return true
	}
	if strings.Count(raw, ":") == 1 && !strings.Contains(raw, "/") {
		host, port, err := net.SplitHostPort(raw)
		return err == nil && host != "" && port != ""
	}
	return false
}

func normalizeListen(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty listen address")
	}

	if barePortPattern.MatchString(raw) {
		return ":" + raw, nil
	}

	if strings.HasPrefix(raw, ":") {
		if _, err := strconv.Atoi(strings.TrimPrefix(raw, ":")); err != nil {
			return "", errors.New("invalid port")
		}
		return raw, nil
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil || host == "" || port == "" {
		return "", errors.New("expected host:port")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("invalid port")
	}
	return raw, nil
}

func normalizeUpstream(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty upstream")
	}

	if barePortPattern.MatchString(raw) {
		return "127.0.0.1:" + raw, nil
	}

	if strings.HasPrefix(raw, ":") {
		port := strings.TrimPrefix(raw, ":")
		if _, err := strconv.Atoi(port); err != nil {
			return "", errors.New("invalid upstream port")
		}
		return "127.0.0.1:" + port, nil
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil || host == "" || port == "" {
		return "", errors.New("expected host:port")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("invalid upstream port")
	}
	return raw, nil
}
