# serve

`serve` is a single-binary dev tool built on embedded Caddy.

It supports:
- static file serving with directory browse mode
- reverse proxy with HTTP and WebSocket support
- compact ngrok-like local request logs

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
serve . --log pretty
serve . --log compact
serve . --log json
serve . --errors-only
serve . --log-file /tmp/serve-access.log
```

## Build

```bash
make build
make run ARGS="."
make test

go build ./cmd/serve
```
