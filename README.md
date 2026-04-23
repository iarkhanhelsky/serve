# serve

`serve` is a single-binary dev tool built on embedded Caddy.

It supports:
- static file serving with directory browse mode
- reverse proxy with HTTP and WebSocket support
- ngrok-style live status screen with request metrics

## Requirements

- Go 1.24+

## Usage

```bash
serve .
serve /path/to/dir
serve :8080
serve 80:8080
serve 0.0.0.0:80=127.0.0.1:8080
```

Argument forms:
- `path` only
- `listen` only
- `listen=upstream`
- `listen:upstream` shorthand alias
- `path + listen`

Normalization rules:
- bare listen port `8080` => `:8080`
- alias `80:8080` => `:80=127.0.0.1:8080`
- bare upstream port `3000` => `127.0.0.1:3000`

## Log modes

```bash
serve . --log status
serve . --log pretty
serve . --log compact
serve . --log json
serve . --errors-only
serve . --log-file /tmp/serve-access.log
```

`status` is the default log mode. In interactive terminals it shows a live, single-panel
status screen instead of printing one line per request. If output is non-interactive,
`status` falls back to compact line output.

## Build

```bash
# Dev build (faster builds, easier debugging)
make build

# Release build (smaller binary)
make build-release

make run ARGS="."
make test

go build ./cmd/serve
```

Size check example:

```bash
ls -lh ./bin/serve
go version -m ./bin/serve | rg "build\\s+-trimpath|build\\s+-ldflags|build\\s+CGO_ENABLED"
```
