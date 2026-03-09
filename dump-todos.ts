import * as http from "http";
import * as https from "https";
import * as fs from "fs";
import * as crypto from "crypto";
import { spawn } from "child_process";

const CLIENT_ID = "3187224c-ea09-4c7f-94bc-b1ba83001a4e";
const TENANT_ID = "518a43e5-ff84-49ea-9a28-73053588b03d";
const SCOPE = "Tasks.Read offline_access";
const REDIRECT_URI = "http://localhost:3000";

// --- Auth ---

function generatePKCE(): { code_verifier: string; code_challenge: string } {
  const code_verifier = crypto.randomBytes(32).toString("base64url");
  const code_challenge = crypto
    .createHash("sha256")
    .update(code_verifier)
    .digest("base64url");
  return { code_verifier, code_challenge };
}

async function getAuthorizationCode(code_challenge: string): Promise<string> {
  return new Promise((resolve, reject) => {
    let timeoutId: NodeJS.Timeout;
    
    const server = http.createServer((req, res) => {
      const url = new URL(`http://localhost${req.url}`);
      const code = url.searchParams.get("code");
      const error = url.searchParams.get("error");

      if (error) {
        res.writeHead(400, { "Content-Type": "text/plain" });
        res.end(`Error: ${error}\n${url.searchParams.get("error_description")}`);
        clearTimeout(timeoutId);
        server.close();
        reject(new Error(`Auth error: ${error}`));
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

    server.listen(3000, () => {
      const authUrl = new URL(`https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/authorize`);
      authUrl.searchParams.set("client_id", CLIENT_ID);
      authUrl.searchParams.set("response_type", "code");
      authUrl.searchParams.set("redirect_uri", REDIRECT_URI);
      authUrl.searchParams.set("scope", SCOPE);
      authUrl.searchParams.set("code_challenge", code_challenge);
      authUrl.searchParams.set("code_challenge_method", "S256");
      authUrl.searchParams.set("prompt", "select_account");

      console.log(`\nOpening browser for authentication...\n`);
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

async function dump(token: string, incompleteOnly: boolean = false): Promise<string> {
  const lists = await getAllPages(token, "/v1.0/me/todo/lists");
  const lines: string[] = [];

  for (const list of lists) {
    const tasks = await getAllPages(token, `/v1.0/me/todo/lists/${list.id}/tasks?$expand=checklistItems`);
    
    // Filter tasks if incompleteOnly is true
    const filteredTasks = incompleteOnly ? tasks.filter(t => t.status !== "completed") : tasks;
    
    // Skip empty lists when filtering
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

// --- Helpers ---

function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)); }

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

(async () => {
  const incompleteOnly = process.argv.includes("--incomplete");
  
  const { code_verifier, code_challenge } = generatePKCE();
  const code = await getAuthorizationCode(code_challenge);
  const token = await exchangeCodeForToken(code, code_verifier);
  console.log(`Authenticated. Fetching tasks${incompleteOnly ? " (incomplete only)" : ""}...`);

  const output = await dump(token, incompleteOnly);
  const outFile = "todo-export.md";
  fs.writeFileSync(outFile, output, "utf8");
  console.log(`Done. Written to ${outFile}`);
  process.exit(0);
})().catch((err) => {
  console.error("Error:", err instanceof Error ? err.message : JSON.stringify(err, null, 2));
  process.exit(1);
});
