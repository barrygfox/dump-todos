import * as http from "http";
import * as https from "https";
import * as fs from "fs";
import * as crypto from "crypto";
import { spawn } from "child_process";
import { pathToFileURL } from "url";

const CLIENT_ID = "3187224c-ea09-4c7f-94bc-b1ba83001a4e";
const TENANT_ID = "518a43e5-ff84-49ea-9a28-73053588b03d";
const SCOPE = "Tasks.Read offline_access";
const REDIRECT_HOST = "127.0.0.1";
const REDIRECT_PORT = 3000;
const REDIRECT_URI = `http://${REDIRECT_HOST}:${REDIRECT_PORT}`;

export type ExportChecklistItem = {
  displayName: string;
  isChecked: boolean;
};

export type ExportTaskBody = {
  contentType?: string;
  content?: string;
};

export type ExportTask = {
  title: string;
  status: string;
  body?: ExportTaskBody;
  dueDateTime?: { dateTime: string };
  checklistItems?: ExportChecklistItem[];
};

export type ExportList = {
  displayName: string;
  tasks: ExportTask[];
};

// --- Auth ---

function generatePKCE(): { code_verifier: string; code_challenge: string } {
  const code_verifier = crypto.randomBytes(32).toString("base64url");
  const code_challenge = crypto
    .createHash("sha256")
    .update(code_verifier)
    .digest("base64url");
  return { code_verifier, code_challenge };
}

function generateState(): string {
  return crypto.randomBytes(32).toString("base64url");
}

async function getAuthorizationCode(code_challenge: string, expectedState: string): Promise<string> {
  return new Promise((resolve, reject) => {
    let timeoutId: NodeJS.Timeout;
    
    const server = http.createServer((req, res) => {
      const url = new URL(req.url ?? "/", REDIRECT_URI);
      const code = url.searchParams.get("code");
      const error = url.searchParams.get("error");
      const state = url.searchParams.get("state");

      if (url.pathname !== "/") {
        res.writeHead(404);
        res.end();
        return;
      }

      if (error) {
        res.writeHead(400, { "Content-Type": "text/plain" });
        res.end(`Error: ${error}\n${url.searchParams.get("error_description")}`);
        clearTimeout(timeoutId);
        server.close();
        reject(new Error(`Auth error: ${error}`));
      } else if (!state || state !== expectedState) {
        res.writeHead(400, { "Content-Type": "text/plain" });
        res.end("Invalid OAuth state");
        clearTimeout(timeoutId);
        server.close();
        reject(new Error("Invalid OAuth state"));
      } else if (code) {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("Authentication successful! You can close this window.");
        clearTimeout(timeoutId);
        server.close();
        resolve(code);
      } else {
        res.writeHead(404);
        res.end();
      }
    });

    server.listen(REDIRECT_PORT, REDIRECT_HOST, () => {
      const authUrl = new URL(`https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/authorize`);
      authUrl.searchParams.set("client_id", CLIENT_ID);
      authUrl.searchParams.set("response_type", "code");
      authUrl.searchParams.set("redirect_uri", REDIRECT_URI);
      authUrl.searchParams.set("scope", SCOPE);
      authUrl.searchParams.set("code_challenge", code_challenge);
      authUrl.searchParams.set("code_challenge_method", "S256");
      authUrl.searchParams.set("state", expectedState);
      authUrl.searchParams.set("prompt", "select_account");

      console.error(`\nOpening browser for authentication...\n`);
      spawn("open", [authUrl.toString()]);
    });

    timeoutId = setTimeout(() => {
      server.close();
      reject(new Error("Authentication timeout"));
    }, 10 * 60 * 1000); // Close after 10 minutes
  });
}

async function exchangeCodeForToken(code: string, code_verifier: string): Promise<string> {
  const body = `grant_type=authorization_code&client_id=${CLIENT_ID}&code=${code}&redirect_uri=${encodeURIComponent(REDIRECT_URI)}&code_verifier=${code_verifier}`;
  const res = await jsonPost(`https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/token`, body);
  if (res.access_token) return res.access_token as string;
  throw new Error(`Token exchange failed: ${res.error}`);
}

// --- Graph API ---

async function graphGet(token: string, path: string): Promise<any> {
  return new Promise((resolve, reject) => {
    const options = {
      hostname: "graph.microsoft.com",
      path,
      method: "GET",
      headers: { 
        Authorization: `Bearer ${token}`, 
        "Content-Type": "application/json",
        "Connection": "close"
      },
    };
    const req = https.get(options, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try {
          const parsed = JSON.parse(data);
          if (parsed.error) reject(new Error(`Graph API error: ${parsed.error.code} – ${parsed.error.message}`));
          else resolve(parsed);
        } catch (e) { reject(e); }
      });
    });
    req.on("error", reject);
  });
}

async function getAllPages(token: string, url: string): Promise<any[]> {
  let items: any[] = [];
  let nextUrl: string | null = url;
  while (nextUrl) {
    const path = nextUrl.startsWith("https://graph.microsoft.com") ? nextUrl.slice("https://graph.microsoft.com".length) : nextUrl;
    const res = await graphGet(token, path);
    if (res.value) items = items.concat(res.value);
    nextUrl = res["@odata.nextLink"] || null;
  }
  return items;
}

// --- Dump ---

async function fetchLists(token: string): Promise<ExportList[]> {
  const lists = await getAllPages(token, "/v1.0/me/todo/lists");
  const exportLists: ExportList[] = [];

  for (const list of lists) {
    const tasks = await getAllPages(token, `/v1.0/me/todo/lists/${list.id}/tasks?$expand=checklistItems`);

    exportLists.push({
      displayName: list.displayName,
      tasks,
    });
  }

  return exportLists;
}

export function renderLists(lists: ExportList[], incompleteOnly: boolean = false): string {
  const lines: string[] = [];

  for (const list of lists) {
    const filteredTasks = incompleteOnly ? list.tasks.filter((task) => task.status !== "completed") : list.tasks;
    if (filteredTasks.length === 0) continue;

    lines.push(`# ${list.displayName}`);

    for (const task of filteredTasks) {
      const done = task.status === "completed" ? "x" : " ";
      const due = task.dueDateTime ? ` (due: ${task.dueDateTime.dateTime.slice(0, 10)})` : "";
      lines.push(`- [${done}] ${task.title}${due}`);
      if (task.body?.content?.trim()) {
        const content = task.body.contentType === "html"
          ? task.body.content.replace(/<[^>]+>/g, " ").replace(/&nbsp;/g, " ").replace(/\s+/g, " ").trim()
          : task.body.content.replace(/\s+/g, " ").trim();
        lines.push(`  - Note: ${content}`);
      }
      if (task.checklistItems?.length) {
        for (const item of task.checklistItems) {
          const sdone = item.isChecked ? "x" : " ";
          lines.push(`  - [${sdone}] ${item.displayName}`);
        }
      }
    }
    lines.push("");
  }

  return lines.join("\n");
}

async function dump(token: string, incompleteOnly: boolean = false): Promise<string> {
  const lists = await fetchLists(token);
  return renderLists(lists, incompleteOnly);
}

// --- Helpers ---

function jsonPost(url: string, body: string): Promise<any> {
  return new Promise((resolve, reject) => {
    const u = new URL(url);
    const options = {
      hostname: u.hostname,
      path: u.pathname + u.search,
      method: "POST",
      headers: { 
        "Content-Type": "application/x-www-form-urlencoded", 
        "Content-Length": Buffer.byteLength(body),
        "Connection": "close"
      },
    };
    const req = https.request(options, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try {
          const parsed = JSON.parse(data);
          if (parsed.error && parsed.error !== "authorization_pending") reject(parsed);
          else resolve(parsed);
        } catch (e) { reject(e); }
      });
    });
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

// --- Main ---

function getFlagValue(flagName: string): string | undefined {
  const flagIndex = process.argv.indexOf(flagName);
  if (flagIndex === -1) return undefined;

  const flagValue = process.argv[flagIndex + 1];
  if (!flagValue || flagValue.startsWith("--")) {
    throw new Error(`${flagName} requires a value`);
  }

  return flagValue;
}

async function main(): Promise<void> {
  const incompleteOnly = process.argv.includes("--incomplete");
  const outputPath = getFlagValue("--output");
  
  const { code_verifier, code_challenge } = generatePKCE();
  const state = generateState();
  const code = await getAuthorizationCode(code_challenge, state);
  const token = await exchangeCodeForToken(code, code_verifier);
  console.error(`Authenticated. Fetching tasks${incompleteOnly ? " (incomplete only)" : ""}...`);

  const output = await dump(token, incompleteOnly);
  if (outputPath) {
    fs.writeFileSync(outputPath, output, { encoding: "utf8", mode: 0o600, flag: "w" });
    console.error(`Done. Written to ${outputPath}`);
    return;
  }

  process.stdout.write(output);
  if (output.length > 0 && !output.endsWith("\n")) {
    process.stdout.write("\n");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  void main().catch((err) => {
    console.error("Error:", err instanceof Error ? err.message : JSON.stringify(err, null, 2));
    process.exit(1);
  });
}
