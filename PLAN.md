# VSCode Plugin — 9router Connector

## Goal
VSCode extension yang bisa konek manual ke 9router/model mana pun, baca selection/active block, eksekusi terminal, dan stream response —seperti Copilot Chat, tapi base engine-nya Go.

## Architecture: Go core + TS shim

```
┌──────────────────────────────────┐
│  VSCode Extension (TypeScript)   │  ← shim tipis, hanya sentuh VSCode API
│  - Baca editor.selection          │
│  - Chat Participant API          │
│  - Spawn Go binary               │
│  - Kirim JSON via stdin          │
└───────────┬──────────────────────┘
            │ stdin/stdout (JSON-RPC, newline-delimited)
┌───────────▼──────────────────────┐
│  Go Core Engine (binary)          │  ← semua logic berat di sini
│  - HTTP client → 9router / model  │
│  - Terminal exec (os/exec)        │
│  - Context building               │
│  - Stream response chunks         │
└──────────────────────────────────┘
```

**Kenapa bukan pure Go?** VSCode extension host = Node.js. Tidak ada extension API native Go.
Cara satu-satunya yang benar-benar jalan: Go binary sebagai child process, TS sebagai jembatan API.
Go kerja semua (HTTP, exec, logic), TS cuma glue code (~200 baris).

## Tech Stack

| Layer        | Teknologi                          |
|--------------|-------------------------------------|
| Extension    | TypeScript + @types/vscode          |
| Build ext    | esbuild (sudah default vscode)      |
| Core engine  | Go (net/http, os/exec, encoding/json) |
| Protocol     | JSON-RPC over stdin/stdout (NDJSON) |
| UI           | VSCode Chat Participant API (native)|
| Config       | VSCode settings (endpoint, api key) |

## Protocol (NDJSON — satu JSON object per baris)

```jsonc
// TS → Go (request)
{"id":1,"method":"chat","params":{"prompt":"jelaskan kode ini","selection":"func main() {}","language":"go","workspacePath":"/home/user/project"}}

// Go → TS (stream chunk)
{"id":1,"method":"chat.chunk","params":{"text":"Fungsi main "}}
{"id":1,"method":"chat.chunk","params":{"text":"adalah entry point"}}
{"id":1,"method":"chat.done","params":{}}

// Go → TS (tool request — minta eksekusi terminal)
{"id":1,"method":"tool.terminal","params":{"command":"go test ./...","requireApproval":true}}

// TS → Go (tool result)
{"id":1,"method":"tool.result","params":{"stdout":"ok\n","exitCode":0}}
```

## Phase 1 — MVP (connect + selection + stream)

- [ ] `go/` — Go binary: baca NDJSON dari stdin, panggil 9router, stream chunk ke stdout
- [ ] `go/` — HTTP client ke 9router (SSE/stream support, configurable endpoint + key)
- [ ] `src/` — TS shim: baca `editor.selection`, kirim ke Go binary, stream response
- [ ] `src/` — Register Chat Participant (`vscode.chat.createChatParticipant`)
- [ ] `src/` — Settings: `9router.endpoint`, `9router.apiKey`, `9router.model`
- [ ] `package.json` — extension manifest, activation events, contributes
- [ ] Build: `go build` untuk binary, `esbuild` untuk extension

## Phase 2 — Terminal execution

- [ ] Go serve mode (long-lived process, bukan spawn per request)
- [ ] Tool-calling: AI minta eksekusi command, Go handle via `os/exec`
- [ ] TS approve modal: user approve/reject command sebelum eksekusi
- [ ] Terminal output dikirim balik ke Go untuk context

## Phase 3 — Context & tools

- [ ] Baca file (Go baca langsung dari workspacePath)
- [ ] Multi-file context
- [ ] Conversation history (Go manage state)
- [ ] Diff/apply edit ke file

## Project structure (minimal)

```
plugin-vscode/
├── PLAN.md              ← this file
├── package.json         ← extension manifest + TS deps
├── tsconfig.json
├── esbuild.js           ← build script (1 file, no config framework)
├── src/
│   └── extension.ts     ← semua TS (selection, spawn, chat participant)
├── go/
│   ├── main.go          ← entry: baca stdin, dispatch method
│   ├── client.go        ← HTTP client ke 9router (stream SSE)
│   ├── terminal.go      ← os/exec wrapper
│   └── protocol.go      ← NDJSON marshal/unmarshal types
├── go.mod
└── README.md
```

## Config (VSCode settings)

```jsonc
{
  "ninerouter.endpoint": "http://localhost:3000/v1/chat/completions",
  "ninerouter.apiKey": "",
  "ninerouter.model": "9router/kombo-cina-free"
}
```

## Build & Run

```bash
# Build Go binary
cd go && go build -o ../bin/engine .

# Build extension
npx esbuild src/extension.ts --bundle --outfile=dist/extension.js --platform=node --external:vscode

# Debug: F5 di VSCode (launch config)
```

## Ceiling notes

- ponytail: Phase 1 pakai spawn-per-request. Upgrade ke serve mode di Phase 2.
- ponytail: Phase 1 tanpa tool-calling. Terminal exec manual di Phase 2.
- ponytail: No conversation memory di Phase 1. Add di Phase 3.
- ponytail: Chat Participant API (bukan custom webview). Native = lebih sedikit kode.
