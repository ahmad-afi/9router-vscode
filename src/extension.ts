import * as vscode from "vscode";
import { spawn, ChildProcess } from "child_process";
import * as path from "path";
import * as fs from "fs";

let proc: ChildProcess | null = null;
let procListeners: ((line: string) => void)[] = [];
let nextId = 1;
const MAX_BUF = 10 * 1024 * 1024;

function getEnginePath(): string {
  const ext = vscode.extensions.getExtension("ahmad-afi.9router-vscode");
  if (ext) {
    const candidate = path.join(ext.extensionPath, "bin", "engine");
    if (fs.existsSync(candidate)) return candidate;
  }
  const devCandidate = path.join(__dirname, "..", "bin", "engine");
  if (fs.existsSync(devCandidate)) return devCandidate;
  return "engine";
}

function ensureProc(): ChildProcess {
  if (proc && !proc.killed) return proc;

  const enginePath = getEnginePath();
  const p = spawn(enginePath, [], { stdio: ["pipe", "pipe", "pipe"] });
  proc = p;

  if (p.stderr) {
    p.stderr.setEncoding("utf8");
    p.stderr.on("data", () => {});
  }

  if (!p.stdout) return p;
  p.stdout.setEncoding("utf8");
  let buf = "";
  p.stdout.on("data", (chunk: string) => {
    buf += chunk;
    if (buf.length > MAX_BUF) {
      buf = "";
      p.kill();
      return;
    }
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

function sendToEngine(msg: Record<string, unknown>): void {
  const p = ensureProc();
  p.stdin?.write(JSON.stringify(msg) + "\n");
}

function sendChatStream(
  params: Record<string, unknown>,
  onChunk: (text: string) => void,
  onToolTerminal: (toolID: number, command: string) => Promise<void>
): Promise<void> {
  return new Promise((resolve, reject) => {
    const p = ensureProc();
    const id = nextId++;
    const req = JSON.stringify({ id, method: "chat", params }) + "\n";

    const timeout = setTimeout(() => {
      procListeners = procListeners.filter((l) => l !== listener);
      reject(new Error("Timeout waiting for Go engine response"));
    }, 120000);

    const listener = async (line: string) => {
      let msg: any;
      try {
        msg = JSON.parse(line);
      } catch {
        return;
      }
      if (msg.id !== id) {
        if (msg.method === "tool.terminal") {
          await handleToolTerminal(msg.id, msg.params);
        } else if (msg.method === "apply.edit") {
          await handleApplyEdit(msg.id, msg.params);
        }
        return;
      }

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

async function handleToolTerminal(toolID: number, params: any): Promise<void> {
  const command: string = params?.command ?? "";
  const workspacePath: string = params?.workspacePath ?? "";

  const choice = await vscode.window.showWarningMessage(
    `Run terminal command?`,
    { modal: true },
    "Run",
    "Cancel"
  );

  if (choice !== "Run") {
    sendToEngine({
      id: toolID,
      method: "tool.result",
      params: { stdout: "", stderr: "Command rejected by user", exitCode: 1 },
    });
    return;
  }

  const term = vscode.window.createTerminal("9router exec");
  term.show();

  if (workspacePath) {
    const sep = process.platform === "win32" ? ";" : "&&";
    term.sendText(`cd "${workspacePath}" ${sep} ${command}`);
  } else {
    term.sendText(command);
  }

  sendToEngine({
    id: toolID,
    method: "tool.result",
    params: {
      stdout: "Command sent to terminal. Check terminal output.",
      stderr: "",
      exitCode: 0,
    },
  });
}

async function handleApplyEdit(toolID: number, params: any): Promise<void> {
  const filePath: string = params?.filePath ?? "";
  const content: string = params?.content ?? "";
  const workspacePath: string =
    vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? "";

  if (!filePath || !workspacePath) {
    sendToEngine({
      id: toolID,
      method: "tool.result",
      params: { stdout: "", stderr: "No file path or workspace", exitCode: 1 },
    });
    return;
  }

  const uri = vscode.Uri.joinPath(vscode.Uri.file(workspacePath), filePath);

  const choice = await vscode.window.showWarningMessage(
    `Apply edit to ${filePath}?`,
    { modal: true },
    "Apply",
    "Cancel"
  );

  if (choice !== "Apply") {
    sendToEngine({
      id: toolID,
      method: "tool.result",
      params: { stdout: "", stderr: "Edit rejected by user", exitCode: 1 },
    });
    return;
  }

  try {
    await vscode.workspace.fs.writeFile(uri, Buffer.from(content, "utf8"));
    sendToEngine({
      id: toolID,
      method: "tool.result",
      params: { stdout: "File written", stderr: "", exitCode: 0 },
    });
  } catch (err: any) {
    sendToEngine({
      id: toolID,
      method: "tool.result",
      params: { stdout: "", stderr: err.message, exitCode: 1 },
    });
  }
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

    const history = chatContext.history
      .filter((h) => h instanceof vscode.ChatRequestTurn || h instanceof vscode.ChatResponseTurn)
      .map((h) => {
        if (h instanceof vscode.ChatRequestTurn) {
          return { role: "user", content: h.prompt || "" };
        }
        const resp = h as vscode.ChatResponseTurn;
        const text = resp.response
          .filter((r): r is vscode.ChatResponseMarkdownPart => r instanceof vscode.ChatResponseMarkdownPart)
          .map((r) => r.value)
          .join("");
        return { role: "assistant", content: text };
      });

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
          history,
        },
        (chunk) => {
          if (!token.isCancellationRequested) {
            stream.markdown(chunk);
          }
        },
        async () => {}
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
