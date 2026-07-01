# 9router VSCode

> VSCode extension with Go core engine — connect manually to 9router or any OpenAI-compatible model, read active selection, execute terminal commands, and stream responses via Chat Participant API.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![VSCode Version](https://img.shields.io/badge/VSCode-1.90+-007ACC?logo=visualstudiocode&logoColor=white)](https://code.visualstudio.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Overview

**9router VSCode** is a Copilot Chat–like experience where you control which model you connect to. Instead of being locked into a single provider, you point the extension at any OpenAI-compatible endpoint (9router, local Ollama, OpenAI, Groq, etc.) and chat with it directly from VSCode's native Chat panel.

The heavy lifting (HTTP calls, terminal execution, context building) is done by a **Go binary**. A thin TypeScript shim bridges the Go binary to VSCode's extension API.

### Why Go + TypeScript?

VSCode's extension host runs on Node.js — there is no native Go binding to the extension API. The only path that actually works:

```
VSCode API (TS)  →  stdin/stdout  →  Go binary (logic)
```

- **TypeScript** (~200 lines): reads editor selection, spawns Go binary, renders Chat Participant UI. Just glue.
- **Go** (all logic): HTTP client, terminal exec, context building, streaming.

No heavy frameworks. No Electron hacks. Just a child process talking NDJSON.

---

## Features

| Feature                        | Status  |
|--------------------------------|---------|
| Connect to any OpenAI-compatible endpoint | Phase 1 |
| Stream responses in VSCode Chat | Phase 1 |
| Read active editor selection     | Phase 1 |
| Configurable endpoint / API key / model | Phase 1 |
| Terminal command execution (with approval) | Phase 2 |
| Multi-file context              | Phase 3 |
| Conversation memory             | Phase 3 |
| Diff/apply file edits           | Phase 3 |

---

## Architecture

```
┌──────────────────────────────────────────┐
│         VSCode Extension (TypeScript)     │
│                                          │
│  • Read editor.selection / active block  │
│  • Chat Participant API (native UI)      │
│  • Spawn Go binary as child process      │
│  • Send JSON requests via stdin          │
│  • Receive streamed chunks from stdout   │
│                                          │
└────────────────┬─────────────────────────┘
                 │  stdin / stdout (NDJSON)
                 │  one JSON object per line
┌────────────────▼─────────────────────────┐
│         Go Core Engine (binary)           │
│                                          │
│  • HTTP client → 9router / model (SSE)   │
│  • Terminal execution (os/exec)          │
│  • Context building from workspace       │
│  • Stream response chunks back           │
│  • Conversation state management         │
│                                          │
└──────────────────────────────────────────┘
```

---

## Protocol (NDJSON)

Communication between TS and Go uses newline-delimited JSON — one JSON object per line.

### Request (TS → Go)

```jsonc
{"id":1,"method":"chat","params":{
  "prompt":"Explain this code",
  "selection":"func main() { fmt.Println(\"hello\") }",
  "language":"go",
  "workspacePath":"/home/user/project"
}}
```

### Stream chunks (Go → TS)

```jsonc
{"id":1,"method":"chat.chunk","params":{"text":"The main function "}}
{"id":1,"method":"chat.chunk","params":{"text":"is the entry point of the program."}}
{"id":1,"method":"chat.done","params":{}}
```

### Tool request — terminal exec (Go → TS)

```jsonc
{"id":1,"method":"tool.terminal","params":{
  "command":"go test ./...",
  "requireApproval":true
}}
```

### Tool result (TS → Go)

```jsonc
{"id":1,"method":"tool.result","params":{"stdout":"ok\n","exitCode":0}}
```

---

## Prerequisites

- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 18+ (for building the TS extension)
- [VSCode](https://code.visualstudio.com/) 1.90+

---

## Installation

### 1. Build the Go engine

```bash
cd go
go build -o ../bin/engine .
```

### 2. Build the extension

```bash
npm install
npm run build
```

### 3. Install in VSCode

**Option A — Debug (F5):**
Open the project in VSCode, press `F5` to launch an Extension Development Host.

**Option B — Package as `.vsix`:**
```bash
npm install -g @vscode/vsce
vsce package
# Install the generated .vsix file:
code --install-extension 9router-vscode-0.0.1.vsix
```

---

## Configuration

Open VSCode Settings (`Ctrl+,` / `Cmd+,`) and search for `9router`:

| Setting                | Default                                      | Description                          |
|------------------------|----------------------------------------------|--------------------------------------|
| `9router.endpoint`     | `http://localhost:3000/v1/chat/completions`  | OpenAI-compatible chat endpoint URL  |
| `9router.apiKey`       | *(empty)*                                    | API key for the endpoint             |
| `9router.model`        | `9router/kombo-cina-free`                    | Model identifier                     |

### settings.json example

```jsonc
{
  "9router.endpoint": "https://api.9router.com/v1/chat/completions",
  "9router.apiKey": "sk-your-key-here",
  "9router.model": "9router/kombo-cina-free"
}
```

### Works with any OpenAI-compatible provider

```jsonc
// Local Ollama
{ "9router.endpoint": "http://localhost:11434/v1/chat/completions", "9router.model": "llama3" }

// OpenAI
{ "9router.endpoint": "https://api.openai.com/v1/chat/completions", "9router.model": "gpt-4o" }

// Groq
{ "9router.endpoint": "https://api.groq.com/openai/v1/chat/completions", "9router.model": "llama-3.3-70b-versatile" }
```

---

## Usage

1. Open a file in VSCode.
2. Highlight a block of code (the extension reads your active selection).
3. Open the Chat panel (`Ctrl+Alt+I` / `Cmd+Alt+I`).
4. Select the **9router** chat participant (type `@9router` in the chat).
5. Ask a question — the response streams in real-time.

---

## Project Structure

```
9router-vscode/
├── PLAN.md              # Development plan & roadmap
├── README.md            # This file
├── package.json         # Extension manifest + TS dependencies
├── tsconfig.json        # TypeScript config
├── esbuild.js           # Build script (single file, no framework)
├── .gitignore
├── .vscode/
│   └── launch.json      # Debug config for F5
├── src/
│   └── extension.ts     # All TS code (selection, spawn, chat participant)
├── go/
│   ├── go.mod           # Go module definition
│   ├── main.go          # Entry: read stdin, dispatch method
│   ├── client.go        # HTTP client → 9router (SSE streaming)
│   ├── terminal.go      # os/exec wrapper for terminal commands
│   └── protocol.go      # NDJSON marshal/unmarshal types
└── bin/
    └── engine           # Compiled Go binary (gitignored)
```

---

## Development

### Build

```bash
# Build Go binary
cd go && go build -o ../bin/engine .

# Build extension (watch mode)
npm run watch

# One-shot build
npm run build
```

### Debug

Open the project in VSCode and press `F5`. This launches an Extension Development Host with the extension loaded.

### Test the Go engine standalone

```bash
echo '{"id":1,"method":"chat","params":{"prompt":"hello","selection":"","language":"text","workspacePath":"."}}' | ./bin/engine
```

---

## Roadmap

### Phase 1 — MVP (connect + selection + stream)
- [ ] Go binary: read NDJSON from stdin, call 9router, stream chunks to stdout
- [ ] HTTP client with SSE streaming support
- [ ] TS shim: read `editor.selection`, spawn Go binary, stream response
- [ ] Register Chat Participant (`vscode.chat.createChatParticipant`)
- [ ] Settings: endpoint, API key, model
- [ ] Build pipeline: `go build` + `esbuild`

### Phase 2 — Terminal execution
- [ ] Go serve mode (long-lived process, not spawn-per-request)
- [ ] Tool-calling: AI requests terminal command execution
- [ ] TS approval modal: user approves/rejects before execution
- [ ] Terminal output sent back to Go for context

### Phase 3 — Context & tools
- [ ] Read files (Go reads directly from workspacePath)
- [ ] Multi-file context
- [ ] Conversation history (Go manages state)
- [ ] Diff/apply edits to files

---

## How It Works

1. **You ask a question** in the VSCode Chat panel.
2. **TS shim** reads your active editor selection and workspace path.
3. **TS shim** spawns (or reuses) the Go binary and sends a chat request via stdin as NDJSON.
4. **Go binary** makes an HTTP request to your configured endpoint (9router or any OpenAI-compatible API) with streaming enabled.
5. **Go binary** streams response chunks back to TS via stdout (NDJSON).
6. **TS shim** renders the streamed chunks in the Chat panel in real-time.

For terminal execution (Phase 2):
7. **Go binary** sends a `tool.terminal` request to TS.
8. **TS shim** shows an approval modal to the user.
9. If approved, **TS shim** executes the command and sends the output back to Go.

---

## Contributing

1. Fork the repo
2. Create a branch (`git checkout -b feature/my-feature`)
3. Commit changes (`git commit -am 'Add my feature'`)
4. Push (`git push origin feature/my-feature`)
5. Open a Pull Request

### Guidelines
- Keep the TS shim thin — logic goes in Go.
- No new dependencies unless absolutely necessary. Stdlib first.
- One file per concern. No premature abstractions.
- Test non-trivial Go logic with a simple `*_test.go` file.

---

## License

[MIT](LICENSE)
