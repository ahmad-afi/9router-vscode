import * as vscode from "vscode";
import { spawn, ChildProcess } from "child_process";
import * as path from "path";
import * as fs from "fs";

let proc: ChildProcess | null = null;
let procListeners: ((line: string) => void)[] = [];
let lineBuffer: string[] = [];

function getEnginePath(): string {
  const extDir = vscode.extensions.getExtension("ahmad-afi.9router-vscode")?.extensionPath;
  if (extDir) {
    const candidate = path.join(extDir, "bin", "engine");
    if (fs.existsSync(candidate)) return candidate;
  }
  // dev mode: look relative to workspace
  const devCandidate = path.join(__dirname, "..", "bin", "engine");
  if (fs.existsSync(devCandidate)) return devCandidate;
  // fallback: look in workspace folder
  const ws = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (ws) {
    const wsCandidate = path.join(ws, "bin", "engine");
    if (fs.existsSync(wsCandidate)) return wsCandidate;
  }
  return "engine";
}

function ensureProc(): ChildProcess {
  if (proc && !proc.killed) return proc;

  const enginePath = getEnginePath();
  const p = spawn(enginePath, [], { stdio: ["pipe", "pipe", "inherit"] });
  proc = p;

  if (!p.stdout) return p;
  p.stdout.setEncoding("utf8");
  let buf = "";
  p.stdout.on("data", (chunk: string) => {
    buf += chunk;
    const lines = buf.split("\n");
    buf = lines.pop() ?? "";
    for (const line of lines) {
      if (line.trim()) {
        for (const listener of procListeners) {
          listener(line);
        }
      }
    }
  });

  p.on("exit", () => {
    if (proc === p) proc = null;
  });

  return p;
}

function sendRequest(
  method: string,
  params: Record<string, unknown>
): Promise<Record<string, any>> {
  return new Promise((resolve, reject) => {
    const p = ensureProc();
    const id = Date.now();
    const req = JSON.stringify({ id, method, params }) + "\n";

    const timeout = setTimeout(() => {
      procListeners = procListeners.filter((l) => l !== listener);
      reject(new Error("Timeout waiting for Go engine response"));
    }, 120000);

    const listener = (line: string) => {
      let msg: any;
      try {
        msg = JSON.parse(line);
      } catch {
        return;
      }
      if (msg.id !== id) return;

      if (msg.method === "chat.chunk" && msg.params?.text) {
        // streaming — don't resolve yet, but we handle streaming differently
        return;
      }
      if (msg.method === "chat.done") {
        clearTimeout(timeout);
        procListeners = procListeners.filter((l) => l !== listener);
        resolve(msg);
        return;
      }
      if (msg.method === "error") {
        clearTimeout(timeout);
        procListeners = procListeners.filter((l) => l !== listener);
        reject(new Error(msg.params?.message ?? "Unknown error"));
        return;
      }
    };

    procListeners.push(listener);
    p.stdin?.write(req);
  });
}

function sendChatStream(
  params: Record<string, unknown>,
  onChunk: (text: string) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const p = ensureProc();
    const id = Date.now();
    const req = JSON.stringify({ id, method: "chat", params }) + "\n";

    const timeout = setTimeout(() => {
      procListeners = procListeners.filter((l) => l !== listener);
      reject(new Error("Timeout waiting for Go engine response"));
    }, 120000);

    const listener = (line: string) => {
      let msg: any;
      try {
        msg = JSON.parse(line);
      } catch {
        return;
      }
      if (msg.id !== id) return;

      if (msg.method === "chat.chunk" && msg.params?.text) {
        onChunk(msg.params.text);
        return;
      }
      if (msg.method === "chat.done") {
        clearTimeout(timeout);
        procListeners = procListeners.filter((l) => l !== listener);
        resolve();
        return;
      }
      if (msg.method === "error") {
        clearTimeout(timeout);
        procListeners = procListeners.filter((l) => l !== listener);
        reject(new Error(msg.params?.message ?? "Unknown error"));
        return;
      }
    };

    procListeners.push(listener);
    p.stdin?.write(req);
  });
}

function getSelection(): { text: string; language: string } {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return { text: "", language: "" };

  const selection = editor.selection;
  const text = editor.document.getText(selection);
  const language =
    editor.document.languageId === "plaintext"
      ? "text"
      : editor.document.languageId;

  return { text, language };
}

function getConfig(): { endpoint: string; apiKey: string; model: string } {
  const cfg = vscode.workspace.getConfiguration("9router");
  return {
    endpoint: cfg.get("endpoint", "http://localhost:3000/v1/chat/completions"),
    apiKey: cfg.get("apiKey", ""),
    model: cfg.get("model", "9router/kombo-cina-free"),
  };
}

export function activate(context: vscode.ExtensionContext) {
  const handler: vscode.ChatRequestHandler = async (
    request: vscode.ChatRequest,
    chatContext: vscode.ChatContext,
    stream: vscode.ChatResponseStream,
    token: vscode.CancellationToken
  ) => {
    const { text, language } = getSelection();
    const { endpoint, apiKey, model } = getConfig();
    const workspacePath =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";

    let prompt = request.prompt;
    if (request.command === "explain" && text) {
      prompt = `Explain this code:\n\n${prompt}`;
    } else if (request.command === "fix" && text) {
      prompt = `Fix the issues in this code:\n\n${prompt}`;
    }

    try {
      await sendChatStream(
        {
          prompt,
          selection: text,
          language,
          workspacePath,
          endpoint,
          apiKey,
          model,
        },
        (chunk) => {
          if (!token.isCancellationRequested) {
            stream.markdown(chunk);
          }
        }
      );
    } catch (err: any) {
      stream.markdown(`**Error:** ${err.message}`);
    }
  };

  const participant = vscode.chat.createChatParticipant(
    "9router.chat",
    handler
  );

  participant.iconPath = new vscode.ThemeIcon("comment-discussion");

  context.subscriptions.push(participant);
}

export function deactivate() {
  if (proc) {
    proc.kill();
    proc = null;
  }
}
