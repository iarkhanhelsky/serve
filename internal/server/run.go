package server

import (
	"context"
	"fmt"

	"github.com/caddyserver/caddy/v2"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

// Run starts Caddy with JSON config and blocks until ctx is canceled.
func Run(ctx context.Context, configJSON []byte) error {
	if err := caddy.Load(configJSON, false); err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}

	<-ctx.Done()
	return caddy.Stop()
}
