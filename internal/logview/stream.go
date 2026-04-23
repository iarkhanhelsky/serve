package logview

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/iarkhanhelsky/serve/internal/types"
)

// Stream tails access log file and prints formatted lines.
func Stream(ctx context.Context, path string, opts types.RunOptions) error {
	if opts.LogMode == "status" && supportsInteractiveDashboard() {
		return streamDashboard(ctx, path, opts)
	}
	return streamLines(ctx, path, opts)
}

func streamLines(ctx context.Context, path string, opts types.RunOptions) error {
	var offset int64

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		f, err := os.Open(path)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return fmt.Errorf("seek log stream: %w", err)
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			evt, err := ParseAccessEvent(line)
			if err != nil {
				continue
			}
			if opts.ErrorsOnly && evt.Status < 400 {
				continue
			}
			fmt.Println(formatByMode(evt, opts.LogMode, opts.Upstream))
		}
		cur, err := f.Seek(0, io.SeekCurrent)
		if err == nil {
			offset = cur
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			return fmt.Errorf("scan log stream: %w", err)
		}
		_ = f.Close()
		time.Sleep(200 * time.Millisecond)
	}
}

func streamDashboard(ctx context.Context, path string, opts types.RunOptions) error {
	events := make(chan AccessEvent, 128)
	errs := make(chan error, 1)
	frameOpts := dashboardFrameOptions{
		colorize: supportsColorOutput(),
		listen:   opts.Listen,
		root:     opts.Root,
		upstream: opts.Upstream,
		mode:     opts.LogMode,
	}

	go streamDashboardEvents(ctx, path, opts, events, errs)
	return runDashboardUI(ctx, frameOpts, events, errs)
}

func streamDashboardEvents(ctx context.Context, path string, opts types.RunOptions, events chan<- AccessEvent, errs chan<- error) {
	defer close(events)
	defer close(errs)
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := os.Open(path)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			errs <- fmt.Errorf("seek log stream: %w", err)
			return
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			evt, err := ParseAccessEvent(line)
			if err != nil {
				continue
			}
			if opts.ErrorsOnly && evt.Status < 400 {
				continue
			}
			select {
			case <-ctx.Done():
				_ = f.Close()
				return
			case events <- evt:
			}
		}
		cur, err := f.Seek(0, io.SeekCurrent)
		if err == nil {
			offset = cur
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			errs <- fmt.Errorf("scan log stream: %w", err)
			return
		}
		_ = f.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

func formatByMode(evt AccessEvent, mode string, configuredUpstream string) string {
	switch mode {
	case "json":
		return RenderJSON(evt)
	case "compact":
		return RenderCompact(evt, configuredUpstream)
	case "status":
		return RenderCompact(evt, configuredUpstream)
	default:
		return RenderPretty(evt, configuredUpstream)
	}
}
