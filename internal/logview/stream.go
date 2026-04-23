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

func formatByMode(evt AccessEvent, mode string, configuredUpstream string) string {
	switch mode {
	case "json":
		return RenderJSON(evt)
	case "compact":
		return RenderCompact(evt, configuredUpstream)
	default:
		return RenderPretty(evt, configuredUpstream)
	}
}
