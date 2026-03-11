# Go Rewrite

This directory contains a parallel Go implementation of the Microsoft To Do exporter.

The existing TypeScript implementation at the repository root remains the reference implementation.

## Goals

- Keep the Go rewrite isolated from the working TypeScript CLI
- Preserve the core export behavior, including `--incomplete`
- Make configuration packaging-friendly by moving it to flags and environment variables

## Layout

- `cmd/dump-todos`: CLI entrypoint
- `internal/auth`: OAuth authorization code flow with PKCE
- `internal/graph`: Microsoft Graph client and pagination
- `internal/export`: Markdown rendering

## Configuration

The Go CLI uses the current TypeScript values as defaults, but every setting can be overridden.

By default it also caches OAuth tokens in the user config directory so repeated runs do not require a browser login every time.

Environment variables:

```bash
export DUMP_TODOS_CLIENT_ID="3187224c-ea09-4c7f-94bc-b1ba83001a4e"
export DUMP_TODOS_TENANT_ID="518a43e5-ff84-49ea-9a28-73053588b03d"
export DUMP_TODOS_SCOPE="Tasks.Read offline_access"
export DUMP_TODOS_REDIRECT_HOST="127.0.0.1"
export DUMP_TODOS_REDIRECT_PORT="3000"
export DUMP_TODOS_TOKEN_CACHE="$HOME/Library/Application Support/dump-todos-go/token-cache.json"
```

Flags:

```bash
go run ./cmd/dump-todos --incomplete
go run ./cmd/dump-todos --output ../todo-export-go.md
go run ./cmd/dump-todos --client-id <client-id> --tenant-id <tenant-id>
go run ./cmd/dump-todos --no-token-cache
```

When `--output` is omitted, the CLI writes Markdown to stdout.

## Token caching

- Cached tokens are stored with owner-only permissions.
- Cache entries are scoped to the client ID, tenant ID, and scope.
- The CLI reuses a valid access token first, then attempts a refresh token grant, and only falls back to interactive login if needed.
- Use `--no-token-cache` to disable caching for a run.

## Build

```bash
go build -o ./bin/dump-todos ./cmd/dump-todos
make build
```

For macOS release binaries:

```bash
make build-macos
```

That script writes separate `arm64` and `amd64` binaries to `dist/`.

## Comparison Workflow

Run the TypeScript exporter from the repository root:

```bash
npx tsx dump-todos.ts --incomplete --output todo-export.md
```

Run the Go exporter from this directory:

```bash
go run ./cmd/dump-todos --incomplete --output ../todo-export-go.md
```

That keeps both outputs side by side for diffing without changing the TypeScript flow.