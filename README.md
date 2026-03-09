# dump-todos

Small TypeScript utility to export Microsoft To Do tasks to Markdown using Microsoft Graph.

## What it does

- Authenticates with Microsoft Entra ID using authorization code flow with PKCE
- Downloads your Microsoft To Do task lists and tasks
- Writes the export to `todo-export.md`
- Supports exporting all tasks or only incomplete tasks

## Requirements

- Node.js
- A Microsoft Entra app registration configured for public client use
- Microsoft Graph delegated permission: `Tasks.Read`
- Redirect URI registered for the app: `http://127.0.0.1:3000`

## Setup

Install dependencies:

```bash
npm install
```

The script uses the Entra application values defined in [dump-todos.ts](dump-todos.ts). Update these constants if you want to use a different app registration:

- `CLIENT_ID`
- `TENANT_ID`

## Usage

Export all tasks:

```bash
npx tsx dump-todos.ts
```

Export only incomplete tasks:

```bash
npx tsx dump-todos.ts --incomplete
```

During authentication, the script opens your browser and waits for the redirect on `127.0.0.1:3000`.

## Output

The export is written to `todo-export.md` in the project directory.

The file is created with owner-only permissions on Unix-like systems.

## Notes

- The script binds the callback listener to `127.0.0.1` only
- The current scope is `Tasks.Read offline_access`
- The utility follows Graph pagination automatically