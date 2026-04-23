package config

import (
	"encoding/json"

	"github.com/iarkhanhelsky/serve/internal/types"
)

// BuildConfigJSON renders an opinionated Caddy JSON config for serve runtime options.
func BuildConfigJSON(opts types.RunOptions, accessLogFile string) ([]byte, error) {
	var routes []map[string]any

	if opts.Upstream != "" {
		routes = append(routes, map[string]any{
			"handle": []map[string]any{
				{
					"handler": "headers",
					"request": map[string]any{
						"set": map[string]any{
							"X-Request-Id": []string{"{http.request.id}"},
						},
					},
					"response": map[string]any{
						"set": map[string]any{
							"X-Request-Id": []string{"{http.request.id}"},
						},
					},
				},
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]any{
						{"dial": opts.Upstream},
					},
				},
			},
		})
	} else {
		routes = append(routes, map[string]any{
			"handle": []map[string]any{
				{
					"handler": "headers",
					"request": map[string]any{
						"set": map[string]any{
							"X-Request-Id": []string{"{http.request.id}"},
						},
					},
					"response": map[string]any{
						"set": map[string]any{
							"X-Request-Id": []string{"{http.request.id}"},
						},
					},
				},
				{
					"handler": "vars",
					"root":    opts.Root,
				},
				{
					"handler": "file_server",
					"browse":  map[string]any{},
				},
			},
		})
	}

	cfg := map[string]any{
		"admin": map[string]any{"disabled": true},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"serve": map[string]any{
						"listen": []string{opts.Listen},
						"logs": map[string]any{
							"default_logger_name": "access",
						},
						"routes": routes,
					},
				},
			},
		},
		"logging": map[string]any{
			"logs": map[string]any{
				"access": map[string]any{
					"level": "INFO",
					"encoder": map[string]any{
						"format": "json",
					},
					"writer": map[string]any{
						"output":   "file",
						"filename": accessLogFile,
					},
				},
			},
		},
	}

	return json.Marshal(cfg)
}
