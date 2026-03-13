# dump-todos

Go CLI to export Microsoft ToDo tasks to Markdown using Microsoft Graph.

## Status

The Go implementation under `go/` is the canonical version of this project.

The TypeScript implementation at the repository root is retained as an earlier prototype and reference point.

## What it does

- Authenticates with Microsoft Entra ID using authorization code flow with PKCE
- Downloads your Microsoft ToDo task lists and tasks
- Writes Markdown to stdout by default, or to a file when requested
- Supports exporting all tasks or only incomplete tasks

## Requirements

- Go 1.22+
- A Microsoft Entra app registration configured for public client use
- Microsoft Graph delegated permission: `Tasks.Read`
- Redirect URI registered for the app: `http://127.0.0.1:3000`

## Setup

Build the Go CLI:

```bash
cd go
go build -o ./bin/dump-todos ./cmd/dump-todos
```

You can also use the Go `Makefile`:

```bash
cd go
make build
```

The Go CLI uses the existing Entra application values as defaults, but every setting can be overridden with flags or environment variables. See `go/README.md` for the full configuration reference.

## Usage

Run the canonical Go CLI from `go/`:

```bash
cd go
go run ./cmd/dump-todos
```

Write only incomplete tasks to stdout:

```bash
cd go
go run ./cmd/dump-todos --incomplete
```

Write to a file explicitly:

```bash
cd go
go run ./cmd/dump-todos --output ../todo-export-go.md
```

During authentication, the CLI opens your browser and waits for the redirect on `127.0.0.1:3000`.

## TypeScript prototype

The repository root still contains the original TypeScript prototype.

It is no longer the primary implementation, but it can still be used for comparison or reference when needed.

The script uses the Entra application values defined in [dump-todos.ts](dump-todos.ts). Update these constants if you want to use a different app registration:

- `CLIENT_ID`
- `TENANT_ID`

Prototype usage:

```bash
npx tsx dump-todos.ts
```

Write only incomplete tasks to stdout:

```bash
npx tsx dump-todos.ts --incomplete
```

Write to a file explicitly:

```bash
npx tsx dump-todos.ts --output todo-export.md
```

## Output

Both implementations write the export to stdout by default.

If `--output` is provided, the file is created with owner-only permissions on Unix-like systems.

## Notes

- The callback listener binds to `127.0.0.1` only
- The current scope is `Tasks.Read offline_access`
- The utility follows Graph pagination automatically